package collectapp

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	"iotserver/internal/models"
	"iotserver/internal/tools/collectengine"
)

type fakeTagServer struct {
	mu      sync.Mutex
	tags    []collectengine.Tag
	updates int
	creates int
	deletes int
	nextId  int64
}

func (f *fakeTagServer) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	f.mu.Lock()
	defer f.mu.Unlock()
	write := func(data any) {
		_ = json.NewEncoder(w).Encode(map[string]any{"code": 200, "message": "ok", "data": data})
	}
	switch {
	case r.Method == http.MethodGet && r.URL.Path == "/api/v1/tags":
		write(f.tags)
	case r.Method == http.MethodPost && r.URL.Path == "/api/v1/tags":
		f.creates++
		tag := f.formTag(r, f.nextId)
		f.tags = append(f.tags, tag)
		write(tag)
	case r.Method == http.MethodPut && strings.HasPrefix(r.URL.Path, "/api/v1/tags/"):
		f.updates++
		id, _ := strconv.ParseInt(strings.TrimPrefix(r.URL.Path, "/api/v1/tags/"), 10, 64)
		tag := f.formTag(r, id)
		for index := range f.tags {
			if f.tags[index].Id == id {
				f.tags[index] = tag
			}
		}
		write(tag)
	case r.Method == http.MethodDelete && strings.HasPrefix(r.URL.Path, "/api/v1/tags/"):
		f.deletes++
		id, _ := strconv.ParseInt(strings.TrimPrefix(r.URL.Path, "/api/v1/tags/"), 10, 64)
		next := f.tags[:0]
		for _, tag := range f.tags {
			if tag.Id != id {
				next = append(next, tag)
			}
		}
		f.tags = next
		write(nil)
	default:
		http.NotFound(w, r)
	}
}

func (f *fakeTagServer) formTag(r *http.Request, id int64) collectengine.Tag {
	_ = r.ParseMultipartForm(1 << 20)
	deviceId, _ := strconv.ParseInt(r.FormValue("deviceId"), 10, 64)
	storage, _ := strconv.ParseBool(r.FormValue("storage"))
	return collectengine.Tag{
		Id:          id,
		DeviceId:    deviceId,
		TagCode:     r.FormValue("tagCode"),
		Address:     r.FormValue("address"),
		DataType:    r.FormValue("dataType"),
		ReadWrite:   r.FormValue("readWrite"),
		ScaleOffset: r.FormValue("scaleOffset"),
		Storage:     storage,
	}
}

type fakeRemoteUpdater struct {
	updates int
	values  map[string]int64
}

func (f *fakeRemoteUpdater) UpdateCollectPointRemoteIds(_ string, remoteIds map[string]int64) error {
	f.updates++
	if f.values == nil {
		f.values = map[string]int64{}
	}
	for id, remoteId := range remoteIds {
		f.values[id] = remoteId
	}
	return nil
}

func newSyncFixture(t *testing.T, tags ...collectengine.Tag) (*service, *collectengine.Client, *fakeTagServer, *fakeRemoteUpdater) {
	t.Helper()
	fake := &fakeTagServer{tags: tags, nextId: 99}
	server := httptest.NewServer(fake)
	t.Cleanup(server.Close)
	return &service{}, collectengine.New(server.URL, 2*time.Second), fake, &fakeRemoteUpdater{}
}

func samplePoint(remoteId int64, address string) models.CollectPointConfig {
	return models.CollectPointConfig{
		Id:         "p1",
		DeviceId:   "17156557",
		TagId:      "temp",
		RemoteId:   remoteId,
		Address:    address,
		DataType:   "float",
		AccessMode: "R",
		Multiplier: 1,
		Offset:     0,
		Storage:    true,
		Enabled:    true,
	}
}

func existingTag(address string) collectengine.Tag {
	return collectengine.Tag{
		Id:          7,
		DeviceId:    9,
		TagCode:     "temp",
		Address:     address,
		DataType:    "float",
		ReadWrite:   "R",
		ScaleOffset: "1/0",
		Storage:     true,
	}
}

func TestSyncDeviceTagsSkipsUnchangedPoint(t *testing.T) {
	svc, client, fake, updater := newSyncFixture(t, existingTag("40001"))
	point := samplePoint(7, "40001")

	mapping, err := svc.syncDeviceTags(client, updater, "17156557", 9, []models.CollectPointConfig{point}, []models.CollectPointConfig{point})
	if err != nil {
		t.Fatalf("sync: %v", err)
	}
	if fake.updates != 0 || fake.creates != 0 || fake.deletes != 0 {
		t.Fatalf("unchanged point should not hit engine: %+v", fake)
	}
	if len(mapping) != 0 || updater.updates != 0 {
		t.Fatalf("unchanged point should not rewrite remote id: mapping=%v writes=%d", mapping, updater.updates)
	}
}

func TestSyncDeviceTagsUpdatesChangedPoint(t *testing.T) {
	svc, client, fake, updater := newSyncFixture(t, existingTag("40001"))
	point := samplePoint(7, "40002")

	_, err := svc.syncDeviceTags(client, updater, "17156557", 9, []models.CollectPointConfig{point}, []models.CollectPointConfig{point})
	if err != nil {
		t.Fatalf("sync: %v", err)
	}
	if fake.updates != 1 || fake.creates != 0 {
		t.Fatalf("changed point should be updated once: %+v", fake)
	}
	if updater.updates != 0 {
		t.Fatalf("remote id did not change, should not rewrite: %d", updater.updates)
	}
}

func TestSyncDeviceTagsCreatesNewPoint(t *testing.T) {
	svc, client, fake, updater := newSyncFixture(t)
	point := samplePoint(0, "40001")

	mapping, err := svc.syncDeviceTags(client, updater, "17156557", 9, nil, []models.CollectPointConfig{point})
	if err != nil {
		t.Fatalf("sync: %v", err)
	}
	if fake.creates != 1 {
		t.Fatalf("want 1 create, got %d", fake.creates)
	}
	if mapping["p1"] != 99 || updater.values["p1"] != 99 {
		t.Fatalf("created tag id should be persisted: mapping=%v values=%v", mapping, updater.values)
	}
}

func TestSyncDeviceTagsDeletesRemovedPoint(t *testing.T) {
	svc, client, fake, updater := newSyncFixture(t, existingTag("40001"))
	oldPoint := samplePoint(7, "40001")

	_, err := svc.syncDeviceTags(client, updater, "17156557", 9, []models.CollectPointConfig{oldPoint}, nil)
	if err != nil {
		t.Fatalf("sync: %v", err)
	}
	if fake.deletes != 1 {
		t.Fatalf("want 1 delete, got %d", fake.deletes)
	}
}

func TestTagMatches(t *testing.T) {
	remote := existingTag("40001")
	desired := collectengine.Tag{DeviceId: 9, TagCode: "temp", Address: "40001", DataType: "float", ReadWrite: "R", ScaleOffset: "1/0", Storage: true}
	if !tagMatches(remote, desired) {
		t.Fatal("expected tags to match")
	}
	desired.Address = "40002"
	if tagMatches(remote, desired) {
		t.Fatal("expected tags to differ")
	}
}
