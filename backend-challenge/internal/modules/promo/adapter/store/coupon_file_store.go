package store

import (
	"bufio"
	"encoding/gob"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/jsuryahyd/food-cart-order-service/internal/common/logging"
	"github.com/jsuryahyd/food-cart-order-service/internal/modules/promo/entities"
)

// Stores coupon codes and their byte offsets in the processed_coupons.txt file.
type CouponIndex map[string]int64

type CouponFileStore struct {
	processedCouponsFilePath      string
	processedCouponsIndexFilePath string
	couponIndex                   CouponIndex // In-memory index of coupons to file offsets
	logger                        *logging.Logger
}

func NewCouponFileStore(processedCouponsFilePath, processedCouponsIndexFilePath string) *CouponFileStore {
	return &CouponFileStore{
		processedCouponsFilePath:      processedCouponsFilePath,
		processedCouponsIndexFilePath: processedCouponsIndexFilePath,
		couponIndex:                   make(CouponIndex),
		logger:                        logging.GetLogger().With("worker", "CouponFileStore"),
	}
}

// Reads the coupon index from the .idx file into memory.
func (s *CouponFileStore) LoadIndex() error {
	s.logger.Infof("Loading coupon index from %s...", s.processedCouponsIndexFilePath)
	file, err := os.Open(s.processedCouponsIndexFilePath)
	if err != nil {
		return fmt.Errorf("failed to open coupon index file %s: %w", s.processedCouponsIndexFilePath, err)
	}
	defer file.Close()

	decoder := gob.NewDecoder(file)
	if err := decoder.Decode(&s.couponIndex); err != nil {
		return fmt.Errorf("failed to decode coupon index from file %s: %w", s.processedCouponsIndexFilePath, err)
	}
	s.logger.Infof("Successfully loaded %d coupons into in-memory index.", len(s.couponIndex))
	return nil
}

// LoadValidCoupons reads all coupons from the processed_coupons.txt file.
// This is primarily used for the initial cache refresh (e.g., loading the 'hot' subset).
// It now leverages the in-memory index for efficiency if needed for partial loading,
// but for a full load, it will still read sequentially.
func (s *CouponFileStore) LoadValidCoupons() ([]entities.Coupon, error) {
	s.logger.Infof("Loading all valid coupons from %s...", s.processedCouponsFilePath)
	file, err := os.Open(s.processedCouponsFilePath)
	if err != nil {
		return nil, fmt.Errorf("failed to open processed coupons file %s: %w", s.processedCouponsFilePath, err)
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	var coupons []entities.Coupon
	for scanner.Scan() {
		coupon := strings.TrimSpace(scanner.Text())
		if coupon != "" {
			coupons = append(coupons, entities.Coupon(coupon))
		}
	}

	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("error reading processed coupons file: %w", err)
	}
	s.logger.Infof("Loaded %d coupons from %s.", len(coupons), s.processedCouponsFilePath)
	return coupons, nil
}

// LookupCouponInFile efficiently checks if a coupon exists in the processed_coupons.txt file
// by using the in-memory index to seek directly to its location.
func (s *CouponFileStore) LookupCouponInFile(couponCode entities.Coupon) (bool, error) {
	offset, ok := s.couponIndex[string(couponCode)]
	if !ok {
		// Coupon not found in the index, so it's not in the file.
		return false, nil
	}

	file, err := os.Open(s.processedCouponsFilePath)
	if err != nil {
		return false, fmt.Errorf("failed to open processed coupons file %s for lookup: %w", s.processedCouponsFilePath, err)
	}
	defer file.Close()

	// Seek to the exact offset where the coupon code's line starts
	_, err = file.Seek(offset, io.SeekStart)
	if err != nil {
		return false, fmt.Errorf("failed to seek to offset %d in file %s: %w", offset, s.processedCouponsFilePath, err)
	}

	reader := bufio.NewReader(file)
	line, err := reader.ReadString('\n') // Read until the next newline
	if err != nil && err != io.EOF {
		return false, fmt.Errorf("failed to read line from file at offset %d in %s: %w", offset, s.processedCouponsFilePath, err)
	}

	// Important: Trim space and newline, then verify it's the exact coupon.
	// This guards against partial reads or issues if the offset points to the middle of a line
	// (though our worker writes one coupon per line, this is good practice).
	readCoupon := strings.TrimSpace(line)

	if readCoupon == string(couponCode) {
		s.logger.Infof("Coupon %s found in file at offset %d.", couponCode, offset)
		return true, nil
	}

	s.logger.Infof("Coupon %s not exactly matched in file (read '%s' at offset %d).", couponCode, readCoupon, offset)
	return false, nil // Mismatch, even if offset was found
}

// GetCouponsForInitialCache loads a specified number of coupons for the initial hot cache.
// This method iterates through the in-memory index to pick a subset of coupons.
// Note: This approach might not pick the 'first N coupons' in file order directly
// if the iteration order of the map is not consistent with file order.
// For truly 'first N', we'd need to re-read from the file's start or store order in index.
// For simplicity, we'll just pick N coupons from the loaded index keys.
func (s *CouponFileStore) GetCouponsForInitialCache(count int) ([]entities.Coupon, error) {
	if s.couponIndex == nil || len(s.couponIndex) == 0 {
		return nil, fmt.Errorf("coupon index is not loaded or is empty")
	}

	var hotCoupons []entities.Coupon
	i := 0
	for coupon := range s.couponIndex {
		if i >= count {
			break
		}
		hotCoupons = append(hotCoupons, entities.Coupon(coupon))
		i++
	}
	s.logger.Infof("Prepared %d coupons for initial hot cache.", len(hotCoupons))
	return hotCoupons, nil
}
