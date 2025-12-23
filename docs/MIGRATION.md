# Migration from Global API

## Quick Migration

If you're upgrading from the previous global function API, here's how to migrate:

### Before (Global Functions)
```go
// Old global API
data, err := binary.Encode(myStruct)
err = binary.Decode(data, &result)
```

### After (Instance API)
```go
// New instance API
tb := binary.New()
data, err := tb.Encode(myStruct)
err = tb.Decode(data, &result)
```

## Benefits of Migration

- **Complete Isolation**: No shared state between different parts of your application
- **Better Testing**: Each test can have its own isolated instance
- **Thread Safety**: Multiple instances can be used safely across goroutines
- **TinyGo Compatible**: Slice-based caching instead of sync.Map for embedded targets

## Common Migration Patterns

### Simple Replacement
```go
// Replace all instances of:
binary.Encode(data)
binary.Decode(data, &result)
binary.EncodeTo(data, &buffer)

// With:
tb := binary.New()
tb.Encode(data)
tb.Decode(data, &result)
tb.EncodeTo(data, &buffer)
```

### Service Integration
```go
type MyService struct {
    tb *binary.Binary
}

func NewMyService() *MyService {
    return &MyService{
        tb: binary.New(), // Instance per service
    }
}
```

### Testing Migration
```go
func TestMyFunction(t *testing.T) {
    // Old way: Global state could interfere
    // data, _ := binary.Encode(testData)

    // New way: Completely isolated
    tb := binary.New()
    data, err := tb.Encode(testData)
    assert.NoError(t, err)
}
```