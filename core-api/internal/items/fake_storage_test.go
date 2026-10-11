//go:build integration

package items_test

import (
	"context"
	"sync"
	"time"

	"github.com/olehmushka/mykomora/core-api/internal/storage"
)

// fakeStorage satisfies storage.Client without talking to any object
// store, standing in for LocalStack in these integration tests. It tracks
// what a test tells it was "uploaded" (via put), so confirmPhoto/
// confirmAttachment's real HeadObject-based verification has something
// genuine to check against — including the negative case, where a test
// deliberately puts a different content-type or size than it declares at
// confirm time.
type fakeStorage struct {
	mu      sync.Mutex
	objects map[string]fakeObject
}

type fakeObject struct {
	contentType string
	size        int64
}

func newFakeStorage() *fakeStorage {
	return &fakeStorage{objects: map[string]fakeObject{}}
}

// put simulates the client's presigned PUT having landed.
func (f *fakeStorage) put(key, contentType string, size int64) {
	f.mu.Lock()
	defer f.mu.Unlock()

	f.objects[key] = fakeObject{contentType: contentType, size: size}
}

func (f *fakeStorage) PresignPut(_ context.Context, key, contentType string, _ int64) (string, map[string]string, time.Time, error) {
	return "https://fake.invalid/" + key, map[string]string{"Content-Type": contentType}, time.Now().Add(time.Minute), nil
}

func (f *fakeStorage) PresignGet(_ context.Context, key string) (string, error) {
	return "https://fake.invalid/" + key, nil
}

func (f *fakeStorage) HeadObject(_ context.Context, key string) (string, int64, error) {
	f.mu.Lock()
	defer f.mu.Unlock()

	obj, ok := f.objects[key]
	if !ok {
		return "", 0, storage.ErrObjectNotFound
	}

	return obj.contentType, obj.size, nil
}

func (f *fakeStorage) DeleteObject(_ context.Context, key string) error {
	f.mu.Lock()
	defer f.mu.Unlock()

	delete(f.objects, key)

	return nil
}

func (f *fakeStorage) ListObjects(context.Context) ([]storage.Object, error) {
	return nil, nil
}
