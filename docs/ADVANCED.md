# Advanced Usage

## Multiple Instance Usage

```go
// Create multiple isolated instances
httpTB := binary.New()
grpcTB := binary.New()
kafkaTB := binary.New()

// Each instance maintains its own cache and pools
httpData, _ := httpTB.Encode(data)
grpcData, _ := grpcTB.Encode(data)
kafkaData, _ := kafkaTB.Encode(data)
```

## Custom Instance with Logging

```go
// Create instance with custom logging for debugging
tb := binary.New(func(msg ...any) {
    log.Printf("Binary Debug: %v", msg)
})

// Use like normal
data, err := tb.Encode(myStruct)
if err != nil {
    log.Printf("Encoding failed: %v", err)
}
```

## Concurrent Usage

```go
tb := binary.New()

// Safe concurrent usage - internal pooling handles synchronization
go func() {
    data, _ := tb.Encode(data1)
    process(data)
}()

go func() {
    data, _ := tb.Encode(data2)
    process(data)
}()
```

## Error Handling

```go
tb := binary.New()

data, err := tb.Encode(myValue)
if err != nil {
    // Handle encoding error
    log.Printf("Encoding failed: %v", err)
}

var result MyType
err = tb.Decode(data, &result)
if err != nil {
    // Handle decoding error
    log.Printf("Decoding failed: %v", err)
}
```

## Multiple Instance Patterns

**Microservices Pattern**: Different services can use separate instances for complete isolation.

```go
type ProtocolManager struct {
    httpBinary  *binary.Binary
    grpcBinary  *binary.Binary
    kafkaBinary *binary.Binary
}

func NewProtocolManager() *ProtocolManager {
    return &ProtocolManager{
        httpBinary:  binary.New(), // Production: no logging
        grpcBinary:  binary.New(),
        kafkaBinary: binary.New(),
    }
}
```

**Concurrent Processing**: Multiple instances can be used safely across goroutines.

```go
// Each goroutine gets its own instance for complete isolation
go func() {
    tb := binary.New()
    data, _ := tb.Encode(data1)
    process(data)
}()

go func() {
    tb := binary.New()
    data, _ := tb.Encode(data2) // Completely independent
    process(data)
}()
```