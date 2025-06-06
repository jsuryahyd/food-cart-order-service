package promoworker

import (
	"bufio"
	"compress/gzip"
	"context"
	"encoding/gob"
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"runtime"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/go-redis/redis/v8"
	"github.com/jsuryahyd/food-cart-order-service/internal/common/config"
	"github.com/jsuryahyd/food-cart-order-service/internal/common/logging"
	messages "github.com/jsuryahyd/food-cart-order-service/internal/modules/promo/adapter/messaging"
)

// CouponCounts will be stored in a map where the key is the coupon code
// and the value is the count of files it appeared in.
type CouponCounts map[string]byte

// CouponIndex stores coupon codes and their byte offsets in the processed_coupons.txt file.
// This type must match the one defined in promo/store/coupon_file_store.go
type CouponIndex map[string]int64

const couponBatchSize = 50000 // alternatively, start high, when facing errors, reduce and increase when it is going fine.

// Worker encapsulates the coupon processing logic.
type Worker struct {
	appConfig           *config.Config
	redisClient         *redis.Client
	redisCounterClient  *redis.Client
	messagePublisher    messages.Publisher
	messageSubscriber   messages.Subscriber // Worker also subscribes to triggers
	couponFileUrls      []string
	outputFilePath      string
	outputIndexFilePath string
	logger              *logging.Logger
}

// NewWorker creates a new Worker instance.
func NewWorker(cfg *config.Config, client *redis.Client, counterClient *redis.Client) *Worker {
	return &Worker{
		appConfig:          cfg,
		redisClient:        client,
		redisCounterClient: counterClient,
		messagePublisher:   messages.NewRedisPublisher(client),
		messageSubscriber:  messages.NewRedisSubscriber(client, cfg.Redis.WorkerTriggerChannel),
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

// CouponCounter defines the interface for different coupon counting strategies.
type CouponCounter interface {
	Increment(ctx context.Context, coupon string) error
	GetAllCounts(ctx context.Context) (CouponCounts, error)
	Reset(ctx context.Context) error // To clear data between runs if needed
}

// InMemoryCouponCounter implements CouponCounter using a local map.
type InMemoryCouponCounter struct {
	counts CouponCounts
	mu     sync.Mutex
	logger *logging.Logger
}

func NewInMemoryCouponCounter(logger *logging.Logger) *InMemoryCouponCounter {
	return &InMemoryCouponCounter{
		counts: make(CouponCounts),
		logger: logger,
	}
}

func (c *InMemoryCouponCounter) Increment(ctx context.Context, coupon string) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.counts[coupon]++
	return nil
}

func (c *InMemoryCouponCounter) GetAllCounts(ctx context.Context) (CouponCounts, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	// Return a copy to prevent external modification, or ensure it's treated as immutable
	// For performance, we'll return the direct map, but a copy might be safer in complex apps.
	return c.counts, nil
}

func (c *InMemoryCouponCounter) Reset(ctx context.Context) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.counts = make(CouponCounts)
	c.logger.Infof("In-memory coupon counts reset.")
	return nil
}

// RedisCouponCounter implements CouponCounter using Redis HASH.
type RedisCouponCounter struct {
	client  *redis.Client
	hashKey string
	logger  *logging.Logger
	ctx     context.Context
}

// NewRedisCouponCounter creates a new RedisCouponCounter.
// hashKey is the Redis HASH key where coupon counts will be stored.
func NewRedisCouponCounter(ctx context.Context, client *redis.Client, hashKey string, logger *logging.Logger) *RedisCouponCounter {
	return &RedisCouponCounter{
		ctx:     ctx,
		client:  client,
		hashKey: hashKey,
		logger:  logger,
	}
}

func (r *RedisCouponCounter) Increment(ctx context.Context, coupon string) error {
	// HINCRBY atomically increments the value associated with a field in a hash.
	// We increment by 1 each time a coupon is seen.
	_, err := r.client.HIncrBy(ctx, r.hashKey, coupon, 1).Result()
	return err
}

// IncrementBatch uses pipelining to send multiple HIncrBy commands.
func (r *RedisCouponCounter) IncrementBatch(ctx context.Context, coupons []string) error {
	if len(coupons) == 0 {
		return nil
	}
	pipe := r.client.Pipeline()
	for _, coupon := range coupons {
		pipe.HIncrBy(ctx, r.hashKey, coupon, 1)
	}
	_, err := pipe.Exec(ctx) // Execute the pipeline
	return err
}

func (r *RedisCouponCounter) GetAllCounts(ctx context.Context) (CouponCounts, error) {
	// HGetAll fetches all fields and values from a hash.
	result, err := r.client.HGetAll(ctx, r.hashKey).Result()
	if err != nil {
		return nil, fmt.Errorf("failed to get all coupon counts from Redis hash %s: %w", r.hashKey, err)
	}

	counts := make(CouponCounts, len(result))
	for coupon, countStr := range result {
		var count int
		_, err := fmt.Sscanf(countStr, "%d", &count) // Parse string count to int
		if err != nil {
			r.logger.Errorf("Failed to parse Redis count for coupon %s: %v", coupon, err)
			continue
		}
		// Redis stores counts as strings, but we're storing them as byte in CouponCounts
		// Ensure the count does not exceed byte max (255)
		if count > 255 {
			counts[coupon] = 255 // Cap at 255 to fit in byte
		} else {
			counts[coupon] = byte(count)
		}
	}
	return counts, nil
}

// StreamValidCoupons iterates over the Redis hash using HSCAN,
// filters coupons based on count (>=2) and length (8-10 chars),
// and sends valid coupons to the provided channel.
// It closes the channel when done.
func (r *RedisCouponCounter) StreamValidCoupons(ctx context.Context, validCouponStream chan<- string) {
	defer close(validCouponStream) // Ensure the channel is closed when this goroutine finishes

	var cursor uint64
	const scanBatchSize = 1000 // Number of elements to fetch per HSCAN call

	r.logger.Infof("Starting to stream valid coupons from Redis hash '%s'...", r.hashKey)

	for {
		// Use HScan to iterate through the hash
		cmd := r.client.HScan(ctx, r.hashKey, cursor, "", int64(scanBatchSize))
		keysAndValues, nextCursor, err := cmd.Result()
		if err != nil {
			r.logger.Errorf("Redis HScan failed while streaming valid coupons: %v", err)
			return // Exit on error
		}

		// keysAndValues slice contains key-value pairs (e.g., ["coupon1", "1", "coupon2", "2"])
		for i := 0; i < len(keysAndValues); i += 2 {
			couponCode := keysAndValues[i]
			countStr := keysAndValues[i+1]

			count, err := strconv.Atoi(countStr) // Convert count string to integer
			if err != nil {
				r.logger.Errorf("Failed to parse Redis count for coupon %s (value: %s): %v", couponCode, countStr, err)
				continue
			}

			// Apply filtering logic:
			// 1. Check if count is at least 2
			// 2. Check coupon code length (8-10 characters)
			if count >= 2 && len(couponCode) >= 8 && len(couponCode) <= 10 {
				select {
				case <-ctx.Done(): // Check context cancellation before sending
					r.logger.Warnf("Context cancelled during Redis HScan stream. Stopping.")
					return
				case validCouponStream <- couponCode: // Send the valid coupon code to the channel
					// Successfully sent
				}
			}
		}

		// Check if the scan is complete
		if nextCursor == 0 {
			break
		}
		cursor = nextCursor // Update cursor for the next iteration

		// Add a small delay between scan calls to avoid overwhelming Redis if needed
		// time.Sleep(10 * time.Millisecond)
	}
	r.logger.Infof("Finished streaming valid coupons from Redis hash '%s'.", r.hashKey)
}

func (r *RedisCouponCounter) Reset(ctx context.Context) error {
	// Delete the entire hash key to clear all counts
	_, err := r.client.Del(ctx, r.hashKey).Result()
	if err != nil {
		return fmt.Errorf("failed to reset Redis coupon counts for hash %s: %w", r.hashKey, err)
	}
	r.logger.Infof("Redis coupon counts for hash %s reset.", r.hashKey)
	return nil
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

	// Determine which coupon counter strategy to use based on config
	var couponCounter CouponCounter
	if w.appConfig.CouponProcessor.UseRedisForCounting { // Assume a config flag exists: UseRedisForCounting bool
		couponCounter = NewRedisCouponCounter(ctx, w.redisCounterClient, w.appConfig.RedisCounter.CouponCountsHashKey, w.logger) // Assume a config for hash key name
		w.logger.Info("Using Redis for coupon counting.")
	} else {
		couponCounter = NewInMemoryCouponCounter(w.logger)
		w.logger.Info("Using in-memory map for coupon counting.")
	}

	// Reset counts before each processing run
	if err := couponCounter.Reset(ctx); err != nil {
		return fmt.Errorf("failed to reset coupon counter: %w", err)
	}

	// Initial processing on startup
	w.logger.Info("Performing initial coupon processing on worker startup...")
	w.processCouponFiles(ctx, couponCounter) // Pass context and the chosen counter
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
		// Reset counts before re-processing
		if err := couponCounter.Reset(ctx); err != nil {
			w.logger.Errorf("Failed to reset coupon counter before re-processing: %v", err)
			// Decide if you want to continue or return error here. For now, continue.
		}
		// Pass context and the chosen counter
		if err := w.processCouponFiles(ctx, couponCounter); err != nil {
			w.logger.Errorf("Coupon processing failed %v", err)
			return err

		}

		w.logger.Info("Coupon re-processing finished.")
	}

	w.logger.Info("Coupon pre-processor worker stopped.")
	return nil
}

// downloads, processes, and writes valid coupon codes, and generates an index.
// Now accepts a CouponCounter interface for flexibility.
func (w *Worker) processCouponFiles(ctx context.Context, counter CouponCounter) error {
	// allCouponCounts is now managed by the CouponCounter interface.
	// The mutex is internal to InMemoryCouponCounter if that's chosen.
	// For Redis, locking is handled by Redis's atomic operations.

	processingCtx, cancel := context.WithCancel(ctx)
	defer cancel()

	// Channel to send batches of coupons from downloadAndProcessFile to the consumer
	couponStream := make(chan []string, len(w.couponFileUrls)*100) // Buffered channel to prevent blocking producers

	var wgDownload sync.WaitGroup // WaitGroup for download and process goroutines
	var wgConsumer sync.WaitGroup // WaitGroup for the coupon consumer goroutine

	// Start goroutines for downloading and processing each file
	for _, url := range w.couponFileUrls {
		wgDownload.Add(1)
		go func(fileURL string) {
			defer func() {
				if r := recover(); r != nil {
					w.logger.Errorf("Panic recovered in file processing goroutine for %s: %v", fileURL, r)
				}
			}()
			defer wgDownload.Done()
			w.logger.Infof("Downloading and processing %s\n", fileURL)
			err := downloadAndProcessFile(processingCtx, fileURL, couponStream) // Pass the channel
			if err != nil {
				if errors.Is(err, context.Canceled) { //
					w.logger.Infof("Processing %s cancelled due to context cancellation: %v\n", fileURL, err)
				} else {
					w.logger.Infof("Error processing %s: %v\n", fileURL, err)
				}
				return
			}
			w.logger.Infof("Finished processing %s\n", fileURL)
		}(url)
	}

	// Start a single consumer goroutine to read from the coupon stream
	wgConsumer.Add(1)

	var processErr error
	go func() {
		for {
			select {
			case <-processingCtx.Done():
				w.logger.Error("File Processing cancelled")
				return
			case batch, ok := <-couponStream:
				if !ok { //channel closed.
					w.logger.Info("File read completed")
					return
				}
				if redisCounter, ok := counter.(*RedisCouponCounter); ok {

					const maxRetries = 3
					backOffDelay := 1 * time.Second

					for i := 0; i < maxRetries; i++ {
						err := redisCounter.IncrementBatch(ctx, batch)
						if err == nil {
							break
						}

						w.logger.Warnf("Failed to increment coupon batch in Redis, Will retry after %d seconds.  Error: %v", backOffDelay, err)
						select {
						case <-processingCtx.Done():
							w.logger.Warnw("File processing cancelled")
							cancel()
							return
						case <-time.After(backOffDelay):
							backOffDelay *= 2
						}
					}

					if err := redisCounter.IncrementBatch(ctx, batch); err != nil {
						w.logger.Errorf("Failed to increment coupon batch in Redis, after %d retries: %v", maxRetries, err)
						processErr = err
						cancel()
					}
				} else {
					// Fallback to single increments for InMemoryCounter or other implementations
					for _, coupon := range batch {
						if err := counter.Increment(ctx, coupon); err != nil {
							w.logger.Errorf("Failed to increment coupon %s count: %v", coupon, err)
						}
					}
				}
			}
		}

	}()
	defer wgConsumer.Done()

	w.logger.Infof("Consumer goroutine finished processing all coupon batches.")
	// Wait for all download and process goroutines to finish
	wgDownload.Wait()
	close(couponStream) // Close the channel once all producers are done

	// Wait for the consumer goroutine to finish processing all data
	wgConsumer.Wait()
	if processErr != nil {
		return processErr
	}

	// --- NEW STREAMING LOGIC FOR GETTING VALID COUNTS AND WRITING TO FILE ---
	if redisCounter, ok := counter.(*RedisCouponCounter); ok {
		validCouponStream := make(chan string, couponBatchSize) // Buffered channel for valid coupons

		var wgStreamToFile sync.WaitGroup
		wgStreamToFile.Add(1)

		// Start a goroutine to stream valid coupons from Redis
		go func() {
			defer wgStreamToFile.Done()
			redisCounter.StreamValidCoupons(processingCtx, validCouponStream) // Start streaming
		}()

		// Write valid coupons to file and generate index directly from the stream
		err := w.writeValidCouponsToFileAndGenerateIndex(validCouponStream)
		if err != nil {
			return fmt.Errorf("error writing valid coupons to file and index from stream: %w", err)
		}

		wgStreamToFile.Wait() // Wait for streaming to finish

	} else {
		// Fallback for InMemoryCouponCounter (or any other non-Redis counter)
		// This still loads all counts into memory, but it's assumed InMemoryCounter
		// is used for smaller datasets or testing.
		allCouponCounts, err := counter.GetAllCounts(processingCtx)
		if err != nil {
			return fmt.Errorf("failed to get all coupon counts from in-memory counter: %w", err)
		}

		validCoupons := []string{}
		for coupon, count := range allCouponCounts {
			if count >= 2 {
				// Apply length checks here too for consistency, as they are part of "valid coupons"
				if len(coupon) >= 8 && len(coupon) <= 10 {
					validCoupons = append(validCoupons, coupon)
				}
			}
		}
		// Write valid coupons to file and generate index for in-memory case (needs new adaptor or modify w.writeValidCouponsToFileAndGenerateIndex)
		// For simplicity, create a channel and send them to the same writer.
		tempValidCouponStream := make(chan string, len(validCoupons))
		for _, coupon := range validCoupons {
			tempValidCouponStream <- coupon
		}
		close(tempValidCouponStream) // Important to close after sending all

		err = w.writeValidCouponsToFileAndGenerateIndex(tempValidCouponStream)
		if err != nil {
			return fmt.Errorf("error writing valid coupons to file and index for in-memory counter: %w", err)
		}
	}

	// Log memory usage (will primarily reflect runtime overhead if Redis is used)
	var m runtime.MemStats
	runtime.ReadMemStats(&m)
	w.logger.Infof("Memory after processing all files and writing output: Alloc = %v MiB, TotalAlloc = %v MiB, Sys = %v MiB, NumGC = %v",
		bToMb(m.Alloc), bToMb(m.TotalAlloc), bToMb(m.Sys), m.NumGC)

	// After processing and writing, notify the main application via Redis that files are updated.
	err := w.messagePublisher.Publish(context.Background(), w.appConfig.Redis.CouponUpdateChannel, "coupons_updated")
	if err != nil {
		w.logger.Infof("Warning: Failed to publish Redis message to %s: %v", w.appConfig.Redis.CouponUpdateChannel, err)
	}
	return nil
}

// downloads a gzipped file, decompresses it,
// and extracts unique coupon codes from it.
func downloadAndProcessFile(ctx context.Context, fileURL string, couponStream chan<- []string) error {
	httpClient := &http.Client{
		// Timeout: 120 * time.Minute,
	}
	// Make sure the HTTP request uses the provided context
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, fileURL, nil)
	if err != nil {
		return fmt.Errorf("failed to create HTTP request: %w", err)
	}

	resp, err := httpClient.Do(req)
	if err != nil {
		// Check for context cancellation explicitly here as well
		if errors.Is(err, context.Canceled) { //
			return ctx.Err()
		}
		return fmt.Errorf("failed to download file: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("failed to download file, status code: %d", resp.StatusCode)
	}
	gzipReader, err := gzip.NewReader(resp.Body)
	if err != nil {
		return fmt.Errorf("failed to create gzip reader: %w", err)
	}
	defer gzipReader.Close()

	scanner := bufio.NewScanner(gzipReader)
	currentBatch := make([]string, 0, couponBatchSize)
	couponsReadInFile := 0

	for scanner.Scan() {
		// Check context cancellation frequently inside the loop
		select {
		case <-ctx.Done():
			return ctx.Err() // Return context error if cancelled
		default:
			// Continue processing
		}

		coupon := strings.TrimSpace(scanner.Text())
		if len(coupon) < 8 || len(coupon) > 10 {
			continue
		}
		if coupon != "" {
			currentBatch = append(currentBatch, coupon)
			couponsReadInFile++ // Not used further, can remove

			if len(currentBatch) >= couponBatchSize {
				select {
				case <-ctx.Done(): // Check before sending to channel
					return ctx.Err()
				case couponStream <- currentBatch: // Send the batch
				}
				currentBatch = make([]string, 0, couponBatchSize)                                                                        // Reset for the next batch
				log.Printf("Sent %d coupons in batch from %s. Total processed in file: %d", couponBatchSize, fileURL, couponsReadInFile) // Too noisy, replaced by logger
			}
		}
	}

	// Send any remaining coupons in the last batch
	if len(currentBatch) > 0 {
		couponStream <- currentBatch
		log.Printf("Sent remaining %d coupons in batch from %s. Total processed in file: %d", len(currentBatch), fileURL, couponsReadInFile)
	}

	if err := scanner.Err(); err != nil {
		return fmt.Errorf("error reading gzipped file: %w", err)
	}

	return nil
}

// ... (Worker Run method - remains unchanged, except for processCouponFiles call)

// (b) Modify `writeValidCouponsToFileAndGenerateIndex` to accept a channel:
// This method will now read valid coupons from a channel.
func (w *Worker) writeValidCouponsToFileAndGenerateIndex(couponsStream <-chan string) error { // Changed parameter
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
	totalValidCouponsWritten := 0

	// Iterate over the channel of valid coupons
	for coupon := range couponsStream { // Changed loop
		offset, err := file.Seek(0, io.SeekCurrent) // Get current byte offset
		if err != nil {
			return fmt.Errorf("failed to get current file offset: %w", err)
		}
		index[coupon] = offset // Store the offset

		_, err = writer.WriteString(coupon + "\n")
		if err != nil {
			return fmt.Errorf("failed to write coupon %s to file: %w", coupon, err)
		}
		totalValidCouponsWritten++
	}

	if err := writer.Flush(); err != nil {
		return fmt.Errorf("failed to flush writer: %w", err)
	}

	encoder := gob.NewEncoder(indexFile)
	if err := encoder.Encode(index); err != nil {
		return fmt.Errorf("failed to encode index to file: %w", err)
	}

	w.logger.Infof("Successfully wrote %d valid coupons to %s and generated index at %s\n", totalValidCouponsWritten, w.outputFilePath, w.outputIndexFilePath) // Log actual count
	return nil
}

func bToMb(b uint64) uint64 {
	return b / 1024 / 1024
}
