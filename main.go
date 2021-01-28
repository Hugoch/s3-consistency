package main

import (
	"bytes"
	"context"
	"crypto/tls"
	"flag"
	"fmt"
	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/google/uuid"
	log "github.com/sirupsen/logrus"
	"io/ioutil"
	"net/http"
	"os"
	"reflect"
	"runtime"
	"sync"
	"syscall"
)

func initializeS3Client(aEndpoint *string, aRegion *string) *s3.Client {
	customResolver := aws.EndpointResolverFunc(func(service, region string) (aws.Endpoint, error) {
		if aEndpoint != nil || aRegion != nil {
			return aws.Endpoint{
				PartitionID:   "aws",
				URL:           *aEndpoint,
				SigningRegion: *aRegion,
			}, nil
		}
		// returning EndpointNotFoundError will allow the service to fallback to it's default resolution
		return aws.Endpoint{}, &aws.EndpointNotFoundError{}
	})

	// Load the Shared AWS Configuration (~/.aws/config)
	config, err := config.LoadDefaultConfig(context.TODO(), config.WithEndpointResolver(customResolver))
	if err != nil {
		log.Fatalf("\n%v", err)
	}

	// disable SSL cert checking. This is DANGEROUS, and only valid for custom endpoint benchmark. Never use in prod
	tr := &http.Transport{
		TLSClientConfig: &tls.Config{InsecureSkipVerify: true},
	}

	httpClient := &http.Client{Transport: tr}

	client := s3.NewFromConfig(config, func(o *s3.Options) {
		o.UsePathStyle = true
		o.UseAccelerate = false
		o.HTTPClient = httpClient
	})

	return client
}

func createRandomFile(client *s3.Client, bucket string, chunkSize int, key *string) string {
	if key == nil {
		u, err := uuid.NewRandom()
		if err != nil {
			log.Fatal("\nCould not generate an UUID. Maybe lacking entropy?")
		}
		k := u.String()
		key = &k
	}
	b := make([]byte, chunkSize)
	r := bytes.NewReader(b)
	log.Debugf("PUT object %s", *key)
	_, err := client.PutObject(context.TODO(),
		&s3.PutObjectInput{
			Bucket: &bucket,
			Key:    key,
			Body:   r,
		})
	if err != nil {
		log.Fatalf("\n%v", err)
	}
	return *key
}

func listAfterDelete(client *s3.Client, bucket string, iterations int, chunkSize int, errors chan int) {
	count := 0
	total := 0
	for i := 0; i < iterations; i++ {
		// create file
		key := createRandomFile(client, bucket, chunkSize, nil)
		// cleanup
		log.Debugf("DELETE object %s", key)
		_, err := client.DeleteObject(context.TODO(),
			&s3.DeleteObjectInput{
				Bucket: &bucket,
				Key:    &key,
			})
		if err != nil {
			log.Fatalf("\nCould not DELETE object %s :: %v", key, err)
		}
		log.Debugf("LIST objects %s", key)
		output, err := client.ListObjectsV2(context.TODO(),
			&s3.ListObjectsV2Input{Bucket: &bucket})
		if err != nil {
			log.Fatalf("\nCould not list bucket %s :: %v", bucket, err)
		}
		found := false
		for _, object := range output.Contents {
			if aws.ToString(object.Key) == key {
				found = true
				break
			}
		}
		if found {
			count++
			log.Debugf("Got a listAfterDelete error, expected %s file is still listed", key)
		}
		total++
	}
	log.Debugf("listAfterDelete %d/%d failed", count, total)
	errors <- count
}

func listAfterCreate(client *s3.Client, bucket string, iterations int, chunkSize int, errors chan int) {
	count := 0
	total := 0
	for i := 0; i < iterations; i++ {
		// create file
		key := createRandomFile(client, bucket, chunkSize, nil)
		log.Debugf("LIST objects %s", key)
		output, err := client.ListObjectsV2(context.TODO(),
			&s3.ListObjectsV2Input{Bucket: &bucket})
		if err != nil {
			log.Fatalf("Could not list bucket %s :: %v", bucket, err)
		}
		found := false
		for _, object := range output.Contents {
			if aws.ToString(object.Key) == key {
				found = true
				break
			}
		}
		if !found {
			count++
			log.Debugf("Got a listAfterCreate error, expected %s file not listed", key)
		}
		// cleanup
		log.Debugf("DELETE object %s", key)
		_, err = client.DeleteObject(context.TODO(),
			&s3.DeleteObjectInput{
				Bucket: &bucket,
				Key:    &key,
			})
		if err != nil {
			log.Fatalf("Could not DELETE object %s :: %v", key, err)
		}
		total++
	}
	log.Debugf("listAfterCreate %d/%d failed", count, total)
	errors <- count
}

func readAfterOverwrite(client *s3.Client, bucket string, iterations int, chunkSize int, errors chan int) {
	count := 0
	total := 0
	for i := 0; i < iterations; i++ {
		// create file
		key := createRandomFile(client, bucket, chunkSize, nil)
		// overwrite it
		_ = createRandomFile(client, bucket, chunkSize+1, &key)
		// read it
		log.Debugf("GET object %s", key)
		obj, err := client.GetObject(context.TODO(),
			&s3.GetObjectInput{
				Bucket: &bucket,
				Key:    &key,
			})
		if err != nil {
			log.Fatalf("Could not GET object %s :: %v", key, err)
		}
		b, err := ioutil.ReadAll(obj.Body)
		if len(b) != chunkSize+1 {
			log.Debugf("Got a readAfterOverwrite error, expected %d bytes, got %d instead", chunkSize+1, len(b))
			count += 1
		}
		// cleanup
		log.Debugf("DELETE object %s", key)
		_, err = client.DeleteObject(context.TODO(),
			&s3.DeleteObjectInput{
				Bucket: &bucket,
				Key:    &key,
			})
		if err != nil {
			log.Fatalf("Could not DELETE object %s :: %v", key, err)
		}
		total++
	}
	errors <- count
}

func readAfterDelete(client *s3.Client, bucket string, iterations int, chunkSize int, errors chan int) {
	count := 0
	total := 0
	for i := 0; i < iterations; i++ {
		key := createRandomFile(client, bucket, chunkSize, nil)
		log.Debugf("DELETE object %s", key)
		_, err := client.DeleteObject(context.TODO(),
			&s3.DeleteObjectInput{
				Bucket: &bucket,
				Key:    &key,
			})
		if err != nil {
			log.Fatalf("Could not DELETE object %s :: %v", key, err)
		}
		log.Debugf("GET object %s", key)
		_, err = client.GetObject(context.TODO(),
			&s3.GetObjectInput{
				Bucket: &bucket,
				Key:    &key,
			})
		if err == nil {
			count++
		}
		total++
	}
	log.Debugf("readAfterDelete %d/%d failed", count, total)
	errors <- count
}

func readAfterCreate(client *s3.Client, bucket string, iterations int, chunkSize int, errors chan int) {
	count := 0
	total := 0
	for i := 0; i < iterations; i++ {
		key := createRandomFile(client, bucket, chunkSize, nil)
		log.Debugf("GET object %s", key)
		_, err := client.GetObject(context.TODO(),
			&s3.GetObjectInput{
				Bucket: &bucket,
				Key:    &key,
			})
		if err != nil {
			count++
		}
		log.Debugf("DELETE object %s", key)
		_, err = client.DeleteObject(context.TODO(),
			&s3.DeleteObjectInput{
				Bucket: &bucket,
				Key:    &key,
			})
		if err != nil {
			log.Fatalf("Could not DELETE object %s :: %v", key,err)
		}
		total++
	}

	log.Debugf("readAfterCreate %d/%d failed", count, total)
	errors <- count
}

func getFunctionName(i interface{}) string {
	return runtime.FuncForPC(reflect.ValueOf(i).Pointer()).Name()
}

func runTest(client *s3.Client, bucket string, fn func(client *s3.Client, bucket string, iterations int, chunkSize int, errors chan int), iterations int, threads int, chunkSize int) int {
	var wg sync.WaitGroup
	wg.Add(threads)
	c := make(chan int, threads)
	errCount := 0
	for i := 0; i < threads; i++ {
		go func() {
			defer wg.Done()
			fn(client, bucket, iterations, chunkSize, c)
		}()
	}
	wg.Wait()
	for i := 0; i < threads; i++ {
		errCount += <-c
	}

	errPct := float32(errCount) / (float32(iterations) * float32(threads)) * 100.0
	if errCount > 0 {
		fmt.Printf("%30s  |     %6d    |  %6d    |   \033[31m%.4f\033[0m\n", getFunctionName(fn), iterations*threads, errCount, errPct)
	} else {
		fmt.Printf("%30s  |     %6d    |  %6d    |   \033[32m%.4f\033[0m\n", getFunctionName(fn), iterations*threads, errCount, errPct)
	}
	return errCount
}

func cleanUp(client *s3.Client, bucket string) {
	log.Debug("Cleaning repo")
	output, err := client.ListObjectsV2(context.TODO(),
		&s3.ListObjectsV2Input{Bucket: &bucket})
	if err != nil {
		log.Fatalf("Could not list bucket %s :: %v", bucket, err)
	}
	for _, object := range output.Contents {
		key := aws.ToString(object.Key)
		_, err = client.DeleteObject(context.TODO(),
			&s3.DeleteObjectInput{
				Bucket: &bucket,
				Key:    &key,
			})
		if err != nil {
			log.Debug(err)
			log.Fatal("Could not cleanup repository")
		}
	}
}

func main() {
	log.SetLevel(log.InfoLevel)

	// check ulimit values, and warn user in case of too low nofile limit
	var noFileLimit syscall.Rlimit
    err := syscall.Getrlimit(syscall.RLIMIT_NOFILE, &noFileLimit)
    if err != nil {
        log.Errorf("Failed to check nofile ulimit :: %v", err)
    }
    if noFileLimit.Max < 10000{
    	log.Warn("Test may fail due to too many open files errors. Increase your nofile ulimit.")
	}

	iterationsFlag := flag.Int("iterations", 5, "Number of iteration per thread per test.")
	threadsFlag := flag.Int("threads", 5, "Number threads per test.")
	chunkSizeFlag := flag.Int("chunk-size", 1, "Size in bytes of created files")
	endpointFlag := flag.String("endpoint", "https://s3.us-east-1.amazonaws.com", "S3 endpoint to use")
	regionFlag := flag.String("region", "us-east-1", "S3 endpoint to use")
	cleanFlag := flag.Bool("clean", false, "Clean bucket")
	bucketFlag := flag.String("bucket", "s3-consistency", "Bucket to use for test")
	flag.Parse()

	client := initializeS3Client(endpointFlag, regionFlag)
	bucketName := *bucketFlag

	headOutput, err := client.HeadBucket(context.TODO(), &s3.HeadBucketInput{
		Bucket: &bucketName,
	})
	if err != nil {
		if headOutput == nil {
			_, err := client.CreateBucket(context.TODO(), &s3.CreateBucketInput{Bucket: &bucketName})
			if err != nil {
				log.Fatalf("\nCould not create bucket :: %v", err)
			}
		}
	}

	iterations := *iterationsFlag
	threads := *threadsFlag
	chunkSize := *chunkSizeFlag
	fmt.Printf("--------------------------------- \033[1;33mSETUP\033[0m ---------------------------------\n\n")
	if *cleanFlag {
		fmt.Printf("Cleaning up repo...\n")
		cleanUp(client, bucketName)
		os.Exit(0)
	}
	fmt.Printf("--------------------------------- \033[1;32mRESULTS\033[0m ---------------------------------\n\n")
	fmt.Printf("\033[1m%d\033[0m iterations per thread with \033[1m%d\033[0m thread(s)\n", iterations, threads)
	fmt.Printf("\033[1m%d bytes\033[0m chunk\n", chunkSize)
	fmt.Printf("%30s  | %10s    |  %6s    | %8s\n", "Test", "Iterations", "Errors", "% Errors")
	runTest(client, bucketName, readAfterDelete, iterations, threads, chunkSize)
	runTest(client, bucketName, readAfterCreate, iterations, threads, chunkSize)
	runTest(client, bucketName, readAfterOverwrite, iterations, threads, chunkSize)
	runTest(client, bucketName, listAfterCreate, iterations, threads, chunkSize)
	runTest(client, bucketName, listAfterDelete, iterations, threads, chunkSize)
	fmt.Printf("\n------------------------------\n")

}
