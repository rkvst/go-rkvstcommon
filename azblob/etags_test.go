package azblob

import (
	"context"
	"fmt"
	"testing"

	"github.com/datatrails/go-datatrails-common/logger"
	"github.com/google/uuid"
)

func uniqueTestName(testName string, t *testing.T) string {
	uid, err := uuid.NewRandom()
	if err != nil {
		t.Fatalf("%v", err)
		return testName
	}
	return fmt.Sprintf("%s-%s", testName, uid.String())
}

// Tests covering the setting and behaviour of etags. Requires the azurite emulator to be running

// Test_PutIfMatch checks the handling of WithETagMatch, which is typically used
// to re-concile racing updates by guaranteeing a single winner.
func TestPutIfMatch(t *testing.T) {

	logger.New("NOOP")
	defer logger.OnExit()

	testName := uniqueTestName("PutIfMatch", t)

	storer, err := NewDev(NewDevConfigFromEnv(), "devcontainer")
	if err != nil {
		t.Fatalf("failed to connect to blob store emulator: %v", err)
	}
	client := storer.GetServiceClient()
	// This will error if it exists and that is fine
	_, err = client.CreateContainer(context.Background(), "devcontainer", nil)
	if err != nil {
		s := err.Error()
		logger.Sugar.Infof("benign err: %v, %s", err, s)
	}

	blobName := fmt.Sprintf("tests/blobs/%s-%d", testName, 1)

	originalValue := []byte("ORIGINAL_VALUE")
	secondValue := []byte("SECOND_VALUE")
	thirdValue := []byte("THIRD_VALUE")

	// establish the original value
	wr, err := storer.Put(context.Background(), blobName, NewBytesReaderCloser(originalValue))
	if err != nil {
		t.Fatalf("failed put original value: %v", err)
	}

	// put the updated value only if we match the original value
	wr2, err := storer.Put(
		context.Background(), blobName, NewBytesReaderCloser(secondValue), WithEtagMatch(*wr.ETag))
	if err != nil {
		t.Fatalf("failed put second value: %v", err)
	}

	// read back only if it matches the new value
	_, err = storer.Reader(context.Background(), blobName, WithEtagMatch(*wr2.ETag))
	if err != nil {
		t.Fatalf("failed to read value with updated ETag: %v", err)
	}

	// expect an error if we use the stale value
	wr3, err := storer.Reader(context.Background(), blobName, WithEtagMatch(*wr.ETag))
	if err == nil {
		t.Fatalf("updated content despite stale etag: %s", wr3.XMsErrorCode)
	}
	// check the error is exactly as we expect
	if !ErrorFromError(err).IsConditionNotMet() {
		t.Fatalf("expected ConditionNotMet err, got: %v", err)
	}

	_, err = storer.Put(
		context.Background(), blobName, NewBytesReaderCloser(thirdValue), WithEtagMatch(*wr.ETag))
	if err == nil {
		t.Fatalf("overwrote second value with wrong etag")
	}
	_, err = storer.Put(
		context.Background(), blobName, NewBytesReaderCloser(thirdValue), WithEtagMatch(*wr2.ETag))
	if err != nil {
		t.Fatalf("failed put third value: %v", err)
	}
}

// Test_ReadIfNoneMatch tests the handling of the WitEtagNoneMatch option
func Test_ReadIfNoneMatch(t *testing.T) {

	logger.New("NOOP")
	defer logger.OnExit()

	testName := uniqueTestName("ReadIfNoneMatch", t)

	storer, err := NewDev(NewDevConfigFromEnv(), "devcontainer")
	if err != nil {
		t.Fatalf("failed to connect to blob store emulator: %v", err)
	}
	client := storer.GetServiceClient()
	// This will error if it exists and that is fine
	_, _ = client.CreateContainer(context.Background(), "devcontainer", nil)

	blobName := fmt.Sprintf("%s-%s", testName, "blob")

	originalValue := []byte("ORIGINAL_VALUE")
	secondValue := []byte("SECOND_VALUE")

	wr, err := storer.Put(context.Background(), blobName, NewBytesReaderCloser(originalValue))
	if err != nil {
		t.Fatalf("failed put original value: %v", err)
	}

	// change the value
	wr2, err := storer.Put(
		context.Background(), blobName, NewBytesReaderCloser(secondValue))
	if err != nil {
		t.Fatalf("failed put second value: %v", err)
	}
	logger.Sugar.Infof("%v", wr2.ETag)

	// check we *fail* to get it when the matching etag is used
	wr3, err := storer.Reader(context.Background(), blobName, WithEtagNoneMatch(*wr2.ETag))
	if err != nil {
		// For reads we _dont_ get an err (as we do for Put's), instead we have to examine the response.
		t.Fatalf("error reading with stale etag: %v", err)
	}
	if !wr3.ConditionNotMet() {
		t.Fatalf("expected ConditionNotMet")
	}

	// check we do get it when the stale etag is used
	wr4, err := storer.Reader(context.Background(), blobName, WithEtagNoneMatch(*wr.ETag))
	if err != nil {
		t.Fatalf("failed to read fresh value predicated on stale etag: %v", err)
	}
	// Note: unless using the If- headers
	if !wr4.Ok() {
		t.Fatalf("expected Ok")
	}

	_, err = storer.Put(context.Background(), blobName, NewBytesReaderCloser(secondValue))
	if err != nil {
		t.Fatalf("failed put second value: %v", err)
	}

	_, err = storer.Put(context.Background(), blobName, NewBytesReaderCloser(originalValue))
	if err != nil {
		t.Fatalf("failed put original value: %v", err)
	}
}
