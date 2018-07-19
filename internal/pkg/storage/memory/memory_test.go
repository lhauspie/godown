package memory

import (
	"errors"
	"reflect"
	"sort"
	"testing"
	"time"

	"github.com/namreg/godown-v2/internal/pkg/storage"
	"github.com/stretchr/testify/assert"
)

func TestNew(t *testing.T) {
	val := storage.NewStringValue("value1")

	valWithTTL := storage.NewStringValue("value2")
	valWithTTL.SetTTL(time.Now().Add(1 * time.Second))

	tests := []struct {
		name                 string
		items                map[storage.Key]*storage.Value
		expectedItems        map[storage.Key]*storage.Value
		expectedItemsWithTTL map[storage.Key]*storage.Value
	}{
		{
			name:                 "empty",
			items:                nil,
			expectedItems:        map[storage.Key]*storage.Value{},
			expectedItemsWithTTL: map[storage.Key]*storage.Value{},
		},
		{
			name:                 "without_ttl",
			items:                map[storage.Key]*storage.Value{"key": val},
			expectedItems:        map[storage.Key]*storage.Value{"key": val},
			expectedItemsWithTTL: map[storage.Key]*storage.Value{},
		},
		{
			name:                 "with_ttl",
			items:                map[storage.Key]*storage.Value{"key": val, "key_with_ttl": valWithTTL},
			expectedItems:        map[storage.Key]*storage.Value{"key": val, "key_with_ttl": valWithTTL},
			expectedItemsWithTTL: map[storage.Key]*storage.Value{"key_with_ttl": valWithTTL},
		},
	}
	for _, tt := range tests {
		strg := New(tt.items)
		assert.Implements(t, new(storage.Storage), strg)
		assert.Equal(t, tt.expectedItems, strg.items)
		assert.Equal(t, tt.expectedItemsWithTTL, strg.itemsWithTTL)
	}
}

func TestStorage_Put(t *testing.T) {
	type args struct {
		key    storage.Key
		setter storage.ValueSetter
	}
	tests := []struct {
		name    string
		args    args
		wantErr bool
	}{
		{
			name: "setter_returns_nil_value_and_nil_err",
			args: args{
				key:    storage.Key("key1"),
				setter: func(*storage.Value) (*storage.Value, error) { return nil, nil },
			},
			wantErr: false,
		},
		{
			name: "setter_returns_nil_value_and_err",
			args: args{
				key:    storage.Key("key1"),
				setter: func(*storage.Value) (*storage.Value, error) { return nil, errors.New("error") },
			},
			wantErr: true,
		},
		{
			name: "setter_returns_value_and_nil_err",
			args: args{
				key:    storage.Key("key1"),
				setter: func(*storage.Value) (*storage.Value, error) { return storage.NewStringValue("test"), nil },
			},
			wantErr: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			strg := &Storage{
				items:        make(map[storage.Key]*storage.Value),
				itemsWithTTL: make(map[storage.Key]*storage.Value),
			}
			if err := strg.Put(tt.args.key, tt.args.setter); (err != nil) != tt.wantErr {
				t.Errorf("Storage.Put() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestStorage_Put_WhenValueShouldBeDeleted(t *testing.T) {
	strg := &Storage{
		items:        make(map[storage.Key]*storage.Value),
		itemsWithTTL: make(map[storage.Key]*storage.Value),
	}
	strg.items[storage.Key("key1")] = storage.NewListValue("value1")
	strg.itemsWithTTL[storage.Key("key1")] = storage.NewListValue("value1")

	assert.NoError(t, strg.Put(storage.Key("key2"), func(*storage.Value) (*storage.Value, error) {
		return nil, nil
	}))

	_, ok := strg.items[storage.Key("key2")]
	assert.False(t, ok)

	_, ok = strg.itemsWithTTL[storage.Key("key2")]
	assert.False(t, ok)
}

func TestStorage_Put_WhenValueShouldBeAdded(t *testing.T) {
	strg := &Storage{
		items:        make(map[storage.Key]*storage.Value),
		itemsWithTTL: make(map[storage.Key]*storage.Value),
	}

	assert.NoError(t, strg.Put(storage.Key("key"), func(*storage.Value) (*storage.Value, error) {
		val := storage.NewStringValue("value")
		val.SetTTL(time.Now().Add(1 * time.Second))
		return val, nil
	}))

	_, ok := strg.items[storage.Key("key")]
	assert.True(t, ok)

	_, ok = strg.itemsWithTTL[storage.Key("key")]
	assert.True(t, ok)
}

func TestStorage_Put_ExpiredKey(t *testing.T) {
	expired := storage.NewStringValue("value")
	expired.SetTTL(time.Now().Add(-1 * time.Second))

	strg := &Storage{
		items:        map[storage.Key]*storage.Value{"expired": expired},
		itemsWithTTL: map[storage.Key]*storage.Value{"expired": expired},
	}

	strg.Put(storage.Key("expired"), func(old *storage.Value) (*storage.Value, error) {
		assert.Nil(t, old)
		return nil, nil
	})
}

func TestStorage_Get(t *testing.T) {
	expired := storage.NewStringValue("expired_value")
	expired.SetTTL(time.Now().Add(-1 * time.Second))

	strg := New(map[storage.Key]*storage.Value{
		"key":         storage.NewStringValue("value"),
		"expired_key": expired,
	})
	tests := []struct {
		name string
		key  storage.Key
		want *storage.Value
		err  error
	}{
		{
			name: "existing_key",
			key:  storage.Key("key"),
			want: storage.NewStringValue("value"),
		},
		{
			name: "not_existing_key",
			key:  storage.Key("not_existing_key"),
			want: nil,
			err:  storage.ErrKeyNotExists,
		},
		{
			name: "expired_key",
			key:  storage.Key("expired_key"),
			want: nil,
			err:  storage.ErrKeyNotExists,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := strg.Get(tt.key)
			if err != nil && err != tt.err {
				t.Errorf("Storage.Get() error = %v, want err %v", err, tt.err)
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("Storage.Get() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestStorage_Del(t *testing.T) {
	strg := Storage{
		items: map[storage.Key]*storage.Value{
			storage.Key("key"):  storage.NewStringValue("value"),
			storage.Key("key2"): storage.NewStringValue("value2"),
		},
		itemsWithTTL: map[storage.Key]*storage.Value{
			storage.Key("key"): storage.NewStringValue("value"),
		},
	}

	assert.NoError(t, strg.Del(storage.Key("key")))

	assert.Equal(
		t,
		map[storage.Key]*storage.Value{
			storage.Key("key2"): storage.NewStringValue("value2"),
		},
		strg.items,
	)

	assert.Equal(
		t,
		map[storage.Key]*storage.Value{},
		strg.itemsWithTTL,
	)
}

func TestStorage_Keys(t *testing.T) {
	expired := storage.NewStringValue("expired_value")
	expired.SetTTL(time.Now().Add(-1 * time.Second))

	strg := Storage{
		items: map[storage.Key]*storage.Value{
			storage.Key("key"):     storage.NewStringValue("value"),
			storage.Key("key2"):    storage.NewStringValue("value2"),
			storage.Key("expired"): expired,
		},
	}
	expected := []storage.Key{storage.Key("key"), storage.Key("key2")}

	actual, err := strg.Keys()

	assert.NoError(t, err)
	assertKeysEquals(t, expected, actual)
}

func TestStorage_All(t *testing.T) {
	strg := Storage{
		items: map[storage.Key]*storage.Value{
			storage.Key("key"):  storage.NewStringValue("value"),
			storage.Key("key2"): storage.NewStringValue("value2"),
		},
	}
	expected := map[storage.Key]*storage.Value{
		storage.Key("key"):  storage.NewStringValue("value"),
		storage.Key("key2"): storage.NewStringValue("value2"),
	}
	actual, err := strg.All()

	assert.NoError(t, err)
	assert.Equal(t, expected, actual)
}

func TestStorage_AllWithTTL(t *testing.T) {
	strg := Storage{
		items: map[storage.Key]*storage.Value{
			storage.Key("key"):  storage.NewStringValue("value"),
			storage.Key("key2"): storage.NewStringValue("value2"),
		},
		itemsWithTTL: map[storage.Key]*storage.Value{
			storage.Key("key2"): storage.NewStringValue("value2"),
		},
	}
	expected := map[storage.Key]*storage.Value{
		storage.Key("key2"): storage.NewStringValue("value2"),
	}
	actual, err := strg.AllWithTTL()

	assert.NoError(t, err)
	assert.Equal(t, expected, actual)
}

func assertKeysEquals(t *testing.T, a, b []storage.Key) bool {
	sort.Slice(a, func(i, j int) bool {
		return a[i] < a[j]
	})

	sort.Slice(b, func(i, j int) bool {
		return b[i] < b[j]
	})

	return assert.Equal(t, a, b)
}
