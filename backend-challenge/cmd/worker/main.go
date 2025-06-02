package main

import (
	"bufio"
	"compress/gzip"
	"fmt"
	"log"
	"net/http"
	"os"
	"strings"
	"sync"

	"github.com/joho/godotenv"
	"github.com/jsuryahyd/food-cart-order-service/internal/common/config"
)

// Coupon codes will be stored in a map where the key is the coupon code
// and the value is the count of files it appeared in.
type CouponCounts map[string]int

var appConfig *config.Config

// init function to load environment variables using Viper
func init() {
	_ = godotenv.Load()
	configPath := os.Getenv("CONFIG_PATH")

	if configPath == "" {
		configPath = "configs/config.yaml"
	}
	cfg, err := config.LoadConfig(configPath)
	if err != nil {
		log.Fatal("Failed to load config on server starup %w", err)
		return
	}
	appConfig = cfg
	// Log the loaded environment variables for debugging
	log.Printf("Loaded COUPON_FILE1_URL: %s", cfg.CouponProcessor.COUPON_FILE1_URL)
	log.Printf("Loaded COUPON_FILE2_URL: %s", cfg.CouponProcessor.COUPON_FILE2_URL)
	log.Printf("Loaded COUPON_FILE3_URL: %s", cfg.CouponProcessor.COUPON_FILE3_URL)
	log.Printf("Loaded OUTPUT_FILE_PATH: %s", cfg.CouponProcessor.OUTPUT_FILE_PATH)
}

func main() {
	if appConfig == nil {
		log.Fatal("app config not available, or not loaded during init.")
		return
	}
	couponFileUrls := []string{
		appConfig.CouponProcessor.COUPON_FILE1_URL,
		appConfig.CouponProcessor.COUPON_FILE2_URL,
		appConfig.CouponProcessor.COUPON_FILE3_URL,
	}
	outputFilePath := appConfig.CouponProcessor.OUTPUT_FILE_PATH

	if outputFilePath == "" {
		log.Fatal("OUTPUT_FILE_PATH environment variable is not set.")
	}
	for _, url := range couponFileUrls {
		if url == "" {
			log.Fatalf("One or more COUPON_FILE_URLs are not set.")
		}
	}

	processCouponFiles(couponFileUrls, outputFilePath)

	//todo: implement polling with redis
	log.Println("Coupon pre-processor worker finished.")
}

// processCouponFiles downloads, processes, and writes valid coupon codes.
func processCouponFiles(urls []string, outputFilePath string) {
	allCouponCounts := make(CouponCounts)
	var mu sync.Mutex // Mutex to protect allCouponCounts during concurrent updates

	var wg sync.WaitGroup
	for _, url := range urls {
		wg.Add(1)
		go func(fileURL string) {
			defer wg.Done()
			log.Printf("Downloading and processing %s\n", fileURL)
			currentFileCoupons, err := downloadAndProcessFile(fileURL)
			if err != nil {
				log.Printf("Error processing %s: %v\n", fileURL, err)
				return
			}

			mu.Lock()
			for coupon := range currentFileCoupons {
				allCouponCounts[coupon]++
			}
			mu.Unlock()
			log.Printf("Finished processing %s\n", fileURL)
		}(url)
	}
	wg.Wait()

	// Filter for coupons appearing in at least two files
	validCoupons := []string{}
	for coupon, count := range allCouponCounts {
		// Rule 1: Must be a string of length between 8 and 10 characters
		// Rule 2: It can be found in at least two files
		if len(coupon) >= 8 && len(coupon) <= 10 && count >= 2 {
			validCoupons = append(validCoupons, coupon)
		}
	}

	err := writeValidCouponsToFile(validCoupons, outputFilePath)
	if err != nil {
		log.Fatalf("Error writing valid coupons to file: %v", err)
	}
	log.Printf("Successfully wrote %d valid coupons to %s\n", len(validCoupons), outputFilePath)
}

// downloadAndProcessFile downloads a gzipped file, decompresses it,
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

// writeValidCouponsToFile writes the slice of valid coupon codes to a file, one per line.
func writeValidCouponsToFile(coupons []string, filePath string) error {
	file, err := os.Create(filePath)
	if err != nil {
		return fmt.Errorf("failed to create output file: %w", err)
	}
	defer file.Close()

	writer := bufio.NewWriter(file)
	for _, coupon := range coupons {
		_, err := writer.WriteString(coupon + "\n")
		if err != nil {
			return fmt.Errorf("failed to write coupon to file: %w", err)
		}
	}
	return writer.Flush()
}
