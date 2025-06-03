package service

import (
	"fmt"

	"github.com/jsuryahyd/food-cart-order-service/internal/common/config"
	"github.com/jsuryahyd/food-cart-order-service/internal/common/logging"
	"github.com/jsuryahyd/food-cart-order-service/internal/modules/promo/adapter/cache"
	"github.com/jsuryahyd/food-cart-order-service/internal/modules/promo/adapter/store"
	"github.com/jsuryahyd/food-cart-order-service/internal/modules/promo/entities"
)

// handles business logic related to promo codes.
type PromoService struct {
	couponCache *cache.CouponCache
	fileStore   *store.CouponFileStore
	appConfig   *config.Config
	logger      *logging.Logger
}

func NewPromoService(couponCache *cache.CouponCache, fileStore *store.CouponFileStore, appConfig *config.Config) *PromoService {
	ps := &PromoService{
		couponCache: couponCache,
		fileStore:   fileStore,
		appConfig:   appConfig,
		logger:      logging.GetLogger().Named("PromoService"),
	}
	return ps
}

/*
*
ValidateCoupon checks if a given coupon code is valid using a tiered approach:
 1. Redis hot cache.
 2. Indexed file cold store.

If found in the cold store, it's promoted to the hot cache (LRU).
*/
func (s *PromoService) ValidateCoupon(couponCode entities.Coupon) (bool, error) {
	s.logger.Debugf("Attempting to validate coupon: %s", couponCode)

	// Step 1: Check Redis Hot Cache
	isCached, err := s.couponCache.IsCouponValid(string(couponCode))
	if err != nil {
		s.logger.Errorf("Error checking coupon %s in Redis hot cache: %v. Falling back to file store.", couponCode, err)
		// If Redis error, still try file store (degraded mode)
	} else if isCached {
		return true, nil
	}

	s.logger.Debugf("Coupon %s not in Redis hot cache. Checking indexed file cold store.", couponCode)

	// Step 2: Check Indexed File Cold Store
	isFoundInFile, err := s.fileStore.LookupCouponInFile(couponCode)
	if err != nil {
		s.logger.Errorf("Error looking up coupon %s in indexed file: %v. Coupon validation failed.", couponCode, err)
		return false, fmt.Errorf("coupon validation failed due to file store error: %w", err)
	}

	if !isFoundInFile {
		s.logger.Debugf("Coupon %s not found in indexed file. Marking as invalid.", couponCode)
		return false, nil
	}

	s.logger.Debugf("Coupon %s found in indexed file. Promoting to Redis hot cache.", couponCode)
	// Step 3: If found in file, promote to Redis hot cache (triggers LRU)
	err = s.couponCache.AddCouponToCache(string(couponCode))
	if err != nil {
		s.logger.Warnf("Warning: Failed to promote coupon %s to Redis hot cache: %v", couponCode, err)
		// just cache update failed, Don't return error here, as validation succeeded.
	}

	return true, nil
}

// explicitly reloads a subset of coupons from the file store into Redis and loads the index.
// This function should be called by an admin API endpoint or on application startup.
func (s *PromoService) RefreshCacheFromFiles() error {
	s.logger.Infoln("Refreshing Redis hot cache from processed coupon file and loading index...")

	// Load the in-memory index first
	err := s.fileStore.LoadIndex()
	if err != nil {
		return fmt.Errorf("failed to load coupon index: %w", err)
	}

	// Load a subset of coupons for the initial hot cache based on config
	hotCoupons, err := s.fileStore.GetCouponsForInitialCache(s.appConfig.CouponProcessor.NumHotCouponsInCache)
	if err != nil {
		return fmt.Errorf("failed to get hot coupons for initial cache: %w", err)
	}

	var hotCouponsStrings []string
	for _, c := range hotCoupons {
		hotCouponsStrings = append(hotCouponsStrings, string(c))
	}

	// Clear existing Redis cache before setting new hot coupons, or use a method that overwrites.
	// For simplicity, SetValidCoupons replaces for now.
	err = s.couponCache.SetValidCoupons(hotCouponsStrings) // This will push the subset to Redis
	if err != nil {
		return fmt.Errorf("failed to set valid coupons in Redis hot cache during refresh: %w", err)
	}
	s.logger.Infoln("Redis hot cache refresh and index loading completed.")
	return nil
}
