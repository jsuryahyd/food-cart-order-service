package promoworker

import (
	"bufio"
	"compress/gzip"
	"context"
	"encoding/gob"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"strings"
	"sync"

	"github.com/go-redis/redis/v8"
	"github.com/jsuryahyd/food-cart-order-service/internal/common/config"
	"github.com/jsuryahyd/food-cart-order-service/internal/common/logging"
	messages "github.com/jsuryahyd/food-cart-order-service/internal/modules/promo/adapter/messaging"
)

// CouponCounts will be stored in a map where the key is the coupon code
// and the value is the count of files it appeared in.
type CouponCounts map[string]int

// CouponIndex stores coupon codes and their byte offsets in the processed_coupons.txt file.
// This type must match the one defined in promo/store/coupon_file_store.go
type CouponIndex map[string]int64

// Worker encapsulates the coupon processing logic.
type Worker struct {
	appConfig           *config.Config
	redisClient         *redis.Client
	messagePublisher    messages.Publisher
	messageSubscriber   messages.Subscriber // Worker also subscribes to triggers
	couponFileUrls      []string
	outputFilePath      string
	outputIndexFilePath string
	logger              *logging.Logger
}

// NewWorker creates a new Worker instance.
func NewWorker(cfg *config.Config, client *redis.Client) *Worker {
	return &Worker{
		appConfig:         cfg,
		redisClient:       client,
		messagePublisher:  messages.NewRedisPublisher(client),
		messageSubscriber: messages.NewRedisSubscriber(client, cfg.Redis.WorkerTriggerChannel),
		couponFileUrls: []string{
			cfg.CouponProcessor.COUPON_FILE1_URL,
			cfg.CouponProcessor.COUPON_FILE2_URL,
			cfg.CouponProcessor.COUPON_FILE3_URL,
		},
		outputFilePath:      cfg.CouponProcessor.OUTPUT_FILE_PATH,
		outputIndexFilePath: cfg.CouponProcessor.OUTPUT_INDEX_FILE_PATH,
		logger:              logging.GetLogger().Named("worker"),
	}
}

// starts the worker's processing loop.
func (w *Worker) Run(ctx context.Context) error {
	if w.outputFilePath == "" {
		return fmt.Errorf("output file path is not set in config")
	}
	if w.outputIndexFilePath == "" {
		return fmt.Errorf("output index file path is not set in config")
	}
	for _, url := range w.couponFileUrls {
		if url == "" {
			return fmt.Errorf("one or more coupon file URLs are not set in config")
		}
	}

	// Initial processing on startup
	w.logger.Info("Performing initial coupon processing on worker startup...")
	w.processCouponFiles()
	w.logger.Info("Initial coupon processing finished.")

	// Start listening to the worker trigger channel
	w.logger.Infof("Worker is subscribing to Redis channel: %s for job triggers...", w.appConfig.Redis.WorkerTriggerChannel)
	msgChannel, err := w.messageSubscriber.Subscribe(ctx, w.appConfig.Redis.WorkerTriggerChannel)
	if err != nil {
		return fmt.Errorf("failed to subscribe to worker trigger channel %s: %w", w.appConfig.Redis.WorkerTriggerChannel, err)
	}
	defer w.messageSubscriber.Close() // Ensure subscription is closed on worker shutdown

	for msg := range msgChannel {
		w.logger.Infof("Received message from Redis channel %s: %s\n", msg.Channel, msg.Payload)
		w.logger.Info("Triggering coupon file re-processing...")
		w.processCouponFiles()
		w.logger.Info("Coupon re-processing finished.")
	}

	w.logger.Info("Coupon pre-processor worker stopped.")
	return nil
}

// downloads, processes, and writes valid coupon codes, and generates an index.
func (w *Worker) processCouponFiles() {
	allCouponCounts := make(CouponCounts)
	var mu sync.Mutex // Mutex to protect allCouponCounts during concurrent updates

	var wg sync.WaitGroup
	for _, url := range w.couponFileUrls {
		wg.Add(1)
		go func(fileURL string) {
			defer wg.Done()
			w.logger.Infof("Downloading and processing %s\n", fileURL)
			currentFileCoupons, err := downloadAndProcessFile(fileURL)
			if err != nil {
				w.logger.Infof("Error processing %s: %v\n", fileURL, err)
				return
			}

			mu.Lock()
			for coupon := range currentFileCoupons {
				allCouponCounts[coupon]++
			}
			mu.Unlock()
			w.logger.Infof("Finished processing %s\n", fileURL)
		}(url)
	}
	wg.Wait()

	// Filter for coupons appearing in at least two files
	validCoupons := []string{}
	for coupon, count := range allCouponCounts {
		if len(coupon) >= 8 && len(coupon) <= 10 && count >= 2 {
			validCoupons = append(validCoupons, coupon)
		}
	}

	// Write valid coupons to file and generate index
	err := w.writeValidCouponsToFileAndGenerateIndex(validCoupons) // Use worker's method
	if err != nil {
		log.Fatalf("Error writing valid coupons to file and index: %v", err)
	}
	w.logger.Infof("Successfully wrote %d valid coupons to %s and generated index at %s\n", len(validCoupons), w.outputFilePath, w.outputIndexFilePath)

	// After processing and writing, notify the main application via Redis that files are updated.
	err = w.messagePublisher.Publish(context.Background(), w.appConfig.Redis.CouponUpdateChannel, "coupons_updated")
	if err != nil {
		w.logger.Infof("Warning: Failed to publish Redis message to %s: %v", w.appConfig.Redis.CouponUpdateChannel, err)
	}
}

// downloads a gzipped file, decompresses it,
// and extracts unique coupon codes from it.
func downloadAndProcessFile(fileURL string) (map[string]struct{}, error) {
	resp, err := http.Get(fileURL)
	if err != nil {
		return nil, fmt.Errorf("failed to download file: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("failed to download file, status code: %d", resp.StatusCode)
	}

	gzipReader, err := gzip.NewReader(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to create gzip reader: %w", err)
	}
	defer gzipReader.Close()

	scanner := bufio.NewScanner(gzipReader)
	fileCoupons := make(map[string]struct{})
	for scanner.Scan() {
		coupon := strings.TrimSpace(scanner.Text())
		if coupon != "" {
			fileCoupons[coupon] = struct{}{}
		}
	}

	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("error reading gzipped file: %w", err)
	}

	return fileCoupons, nil
}

// writes the slice of valid coupon codes to a file, one per line,
// and simultaneously generates a gob-encoded index file mapping coupons to their byte offsets.
func (w *Worker) writeValidCouponsToFileAndGenerateIndex(coupons []string) error {
	// Create output directory if it doesn't exist
	outputDir := "./" // Default to current directory if no path separator
	if idx := strings.LastIndexByte(w.outputFilePath, '/'); idx != -1 {
		outputDir = w.outputFilePath[:idx]
	}
	if err := os.MkdirAll(outputDir, 0755); err != nil {
		return fmt.Errorf("failed to create output directory %s: %w", outputDir, err)
	}

	file, err := os.Create(w.outputFilePath)
	if err != nil {
		return fmt.Errorf("failed to create output file %s: %w", w.outputFilePath, err)
	}
	defer file.Close()

	indexFile, err := os.Create(w.outputIndexFilePath)
	if err != nil {
		return fmt.Errorf("failed to create index file %s: %w", w.outputIndexFilePath, err)
	}
	defer indexFile.Close()

	writer := bufio.NewWriter(file)
	index := make(CouponIndex)
	for _, coupon := range coupons {
		offset, err := file.Seek(0, io.SeekCurrent) // Get current byte offset
		if err != nil {
			return fmt.Errorf("failed to get current file offset: %w", err)
		}
		index[coupon] = offset // Store the offset

		_, err = writer.WriteString(coupon + "\n")
		if err != nil {
			return fmt.Errorf("failed to write coupon %s to file: %w", coupon, err)
		}
	}

	if err := writer.Flush(); err != nil {
		return fmt.Errorf("failed to flush writer: %w", err)
	}

	encoder := gob.NewEncoder(indexFile)
	if err := encoder.Encode(index); err != nil {
		return fmt.Errorf("failed to encode index to file: %w", err)
	}

	return nil
}
