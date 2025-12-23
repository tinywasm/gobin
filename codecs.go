package binary

import (
	"encoding"
	"reflect"

	. "github.com/tinywasm/fmt"
)

var (
	binaryMarshalerType   = reflect.TypeOf((*encoding.BinaryMarshaler)(nil)).Elem()
	binaryUnmarshalerType = reflect.TypeOf((*encoding.BinaryUnmarshaler)(nil)).Elem()
)

// codec represents a single part codec, which can encode and decode something.
type codec interface {
	encodeTo(*encoder, reflect.Value) error
	decodeTo(*decoder, reflect.Value) error
}

// ------------------------------------------------------------------------------

type reflectArraycodec struct {
	elemcodec codec // The codec of the array's elements
}

// Encode encodes a value into the encoder.
func (c *reflectArraycodec) encodeTo(e *encoder, rv reflect.Value) (err error) {
	l := rv.Len()
	for i := 0; i < l; i++ {
		if err = c.elemcodec.encodeTo(e, rv.Index(i)); err != nil {
			return err
		}
	}
	return e.err
}

// ------------------------------------------------------------------------------

type binaryMarshalercodec struct{}

func (c *binaryMarshalercodec) encodeTo(e *encoder, rv reflect.Value) (err error) {
	// If this is a nil pointer, encode as zero-length payload
	if rv.Kind() == reflect.Ptr && rv.IsNil() {
		e.writeUvarint(0)
		return e.err
	}

	// Ensure we have a value that implements BinaryMarshaler (addr if needed)
	m, ok := rv.Interface().(encoding.BinaryMarshaler)
	if !ok {
		if !rv.CanAddr() {
			return Errf("value of type %s is not addressable and does not implement encoding.BinaryMarshaler", rv.Type())
		}
		rv = rv.Addr()
		m, ok = rv.Interface().(encoding.BinaryMarshaler)
		if !ok {
			return Errf("value of type %s does not implement encoding.BinaryMarshaler", rv.Type())
		}
	}

	b, err := m.MarshalBinary()
	if err != nil {
		return err
	}

	e.writeUvarint(uint64(len(b)))
	if len(b) > 0 {
		e.write(b)
	}
	return err
}

func (c *binaryMarshalercodec) decodeTo(d *decoder, rv reflect.Value) error {
	// Read length-prefixed payload and pass to UnmarshalBinary
	l, err := d.readUvarint()
	if err != nil {
		return err
	}

	var b []byte
	if l > 0 {
		b = make([]byte, int(l))
		if _, err = d.read(b); err != nil {
			return err
		}
	}

	// Ensure we have an addressable value that implements BinaryUnmarshaler
	if rv.Kind() != reflect.Ptr {
		rv = rv.Addr()
	}

	if rv.IsNil() {
		rv.Set(reflect.New(rv.Type().Elem()))
	}

	u, ok := rv.Interface().(encoding.BinaryUnmarshaler)
	if !ok {
		return Errf("value of type %s does not implement encoding.BinaryUnmarshaler", rv.Type())
	}

	return u.UnmarshalBinary(b)
}

// Decode decodes into a reflect value from the decoder.
func (c *reflectArraycodec) decodeTo(d *decoder, rv reflect.Value) (err error) {
	l := rv.Len()
	for i := 0; i < l; i++ {
		idx := rv.Index(i)
		// Don't use Indirect here - use the indexed value directly
		if err = c.elemcodec.decodeTo(d, idx); err != nil {
			return err
		}
	}
	return err
}

// ------------------------------------------------------------------------------

type reflectSlicecodec struct {
	elemcodec codec // The codec of the slice's elements
}

// Encode encodes a value into the encoder.
func (c *reflectSlicecodec) encodeTo(e *encoder, rv reflect.Value) (err error) {
	l := rv.Len()
	e.writeUvarint(uint64(l))
	for i := 0; i < l; i++ {
		if err = c.elemcodec.encodeTo(e, rv.Index(i)); err != nil {
			return err
		}
	}
	return e.err
}

// Decode decodes into a reflect value from the decoder.
func (c *reflectSlicecodec) decodeTo(d *decoder, rv reflect.Value) (err error) {
	var l uint64
	if l, err = d.readUvarint(); err == nil && l > 0 {
		typ := rv.Type()
		newSlice := reflect.MakeSlice(typ, int(l), int(l))
		rv.Set(newSlice)

		for i := 0; i < int(l); i++ {
			idx := rv.Index(i)
			v := reflect.Indirect(idx)
			if err = c.elemcodec.decodeTo(d, v); err != nil {
				return err
			}
		}
	}
	return err
}

// ------------------------------------------------------------------------------

type reflectSliceOfPtrcodec struct {
	elemcodec codec // The codec of the slice's elements
	elemType  reflect.Type
}

// Encode encodes a value into the encoder.
func (c *reflectSliceOfPtrcodec) encodeTo(e *encoder, rv reflect.Value) (err error) {
	l := rv.Len()
	e.writeUvarint(uint64(l))
	for i := 0; i < l; i++ {
		v := rv.Index(i)
		isNil := v.IsNil()
		e.writeBool(isNil)
		if !isNil {
			if err = c.elemcodec.encodeTo(e, v.Elem()); err != nil {
				return err
			}
		}
	}
	return e.err
}

// Decode decodes into a reflect value from the decoder.
func (c *reflectSliceOfPtrcodec) decodeTo(d *decoder, rv reflect.Value) (err error) {
	var l uint64
	var isNil bool
	if l, err = d.readUvarint(); err == nil && l > 0 {
		typ := rv.Type()
		newSlice := reflect.MakeSlice(typ, int(l), int(l))
		rv.Set(newSlice)
		for i := 0; i < int(l); i++ {
			if isNil, err = d.readBool(); !isNil {
				if err != nil {
					return err
				}

				ptr := rv.Index(i)
				// Create new pointer value and decode directly to it
				newPtr := reflect.New(c.elemType)
				indirect := reflect.Indirect(newPtr)
				if err = c.elemcodec.decodeTo(d, indirect); err != nil {
					return err
				}
				// Now copy the decoded value to the slice element
				ptr.Set(newPtr)
			}
		}
	}
	return err
}

// ------------------------------------------------------------------------------

type byteSlicecodec struct{}

// Encode encodes a value into the encoder.
func (c *byteSlicecodec) encodeTo(e *encoder, rv reflect.Value) (err error) {
	l := rv.Len()

	e.writeUvarint(uint64(l))
	if l > 0 {
		e.write(rv.Bytes())
	}
	return err
}

// Decode decodes into a reflect value from the decoder.
func (c *byteSlicecodec) decodeTo(d *decoder, rv reflect.Value) (err error) {
	var l uint64
	if l, err = d.readUvarint(); err == nil && l > 0 {
		var b []byte
		if b, err = d.slice(int(l)); err == nil {
			rv.SetBytes(b)
		}
	}
	return err
}

// ------------------------------------------------------------------------------

type boolSlicecodec struct{}

// Encode encodes a value into the encoder.
func (c *boolSlicecodec) encodeTo(e *encoder, rv reflect.Value) (err error) {
	l := rv.Len()
	e.writeUvarint(uint64(l))
	for i := 0; i < l; i++ {
		e.writeBool(rv.Index(i).Bool())
	}
	return err
}

// Decode decodes into a reflect value from the decoder.
func (c *boolSlicecodec) decodeTo(d *decoder, rv reflect.Value) (err error) {
	var l uint64
	if l, err = d.readUvarint(); err == nil && l > 0 {
		newSlice := reflect.MakeSlice(rv.Type(), int(l), int(l))
		rv.Set(newSlice)
		for i := 0; i < int(l); i++ {
			var b bool
			if b, err = d.readBool(); err == nil {
				rv.Index(i).SetBool(b)
			} else {
				return err
			}
		}
	}
	return err
}

// ------------------------------------------------------------------------------

// numericSlicecodec handles both signed and unsigned numeric slices
type numericSlicecodec struct {
	signed bool
}

// Encode encodes a value into the encoder.
func (c *numericSlicecodec) encodeTo(e *encoder, rv reflect.Value) (err error) {
	l := rv.Len()
	e.writeUvarint(uint64(l))
	if c.signed {
		for i := 0; i < l; i++ {
			e.writeVarint(rv.Index(i).Int())
		}
	} else {
		for i := 0; i < l; i++ {
			e.writeUvarint(rv.Index(i).Uint())
		}
	}
	return e.err
}

// Decode decodes into a reflect value from the decoder.
func (c *numericSlicecodec) decodeTo(d *decoder, rv reflect.Value) (err error) {
	var l uint64
	if l, err = d.readUvarint(); err == nil && l > 0 {
		typ := rv.Type()
		newSlice := reflect.MakeSlice(typ, int(l), int(l))
		rv.Set(newSlice)
		for i := 0; i < int(l); i++ {
			if c.signed {
				v, err := d.readVarint()
				if err != nil {
					return err
				}
				rv.Index(i).SetInt(v)
			} else {
				v, err := d.readUvarint()
				if err != nil {
					return err
				}
				rv.Index(i).SetUint(v)
			}
		}
	}
	return err
}

// ------------------------------------------------------------------------------

type reflectPointercodec struct {
	elemcodec codec
}

// Encode encodes a value into the encoder.
func (c *reflectPointercodec) encodeTo(e *encoder, rv reflect.Value) (err error) {
	if rv.IsNil() {
		e.writeBool(true)
		return err
	}

	e.writeBool(false)
	return c.elemcodec.encodeTo(e, rv.Elem())
}

// Decode decodes into a reflect value from the decoder.
func (c *reflectPointercodec) decodeTo(d *decoder, rv reflect.Value) (err error) {
	isNil, err := d.readBool()
	if err != nil {
		return err
	}
	if isNil {
		return err
	}

	// Check if the pointer is nil and create a new value if needed
	if rv.IsNil() {
		typ := rv.Type()
		// Get the element type using the Type.Elem() method
		elemType := typ.Elem()
		newPtr := reflect.New(elemType)
		rv.Set(newPtr)
	}

	elem := rv.Elem()
	return c.elemcodec.decodeTo(d, elem)
}

// ------------------------------------------------------------------------------

type reflectStructcodec []fieldcodec

type fieldcodec struct {
	Index int   // The index of the field
	codec codec // The codec to use for this field
}

// Encode encodes a value into the encoder.
func (c reflectStructcodec) encodeTo(e *encoder, rv reflect.Value) (err error) {
	for _, i := range c {
		if err = i.codec.encodeTo(e, rv.Field(i.Index)); err != nil {
			return err
		}
	}
	return e.err
}

// Decode decodes into a reflect value from the decoder.
func (c reflectStructcodec) decodeTo(d *decoder, rv reflect.Value) (err error) {
	for _, f := range c {
		if err = f.codec.decodeTo(d, rv.Field(f.Index)); err != nil {
			return err
		}
	}
	return err
}

// ------------------------------------------------------------------------------

type stringcodec struct{}

// Encode encodes a value into the encoder.
func (c *stringcodec) encodeTo(e *encoder, rv reflect.Value) (err error) {
	s := rv.String()
	e.writeString(s)
	return e.err
}

// Decode decodes into a reflect value from the decoder.
func (c *stringcodec) decodeTo(d *decoder, rv reflect.Value) (err error) {
	var s string
	if s, err = d.readString(); err == nil {
		rv.SetString(s)
	}
	return err
}

// ------------------------------------------------------------------------------

type boolcodec struct{}

// Encode encodes a value into the encoder.
func (c *boolcodec) encodeTo(e *encoder, rv reflect.Value) (err error) {
	boolVal := rv.Bool()
	e.writeBool(boolVal)
	return e.err
}

// Decode decodes into a reflect value from the decoder.
func (c *boolcodec) decodeTo(d *decoder, rv reflect.Value) (err error) {
	var out bool
	if out, err = d.readBool(); err == nil {
		rv.SetBool(out)
	}
	return err
}

// ------------------------------------------------------------------------------

type varintcodec struct{}

// Encode encodes a value into the encoder.
func (c *varintcodec) encodeTo(e *encoder, rv reflect.Value) (err error) {
	intVal := rv.Int()
	e.writeVarint(intVal)
	return e.err
}

// Decode decodes into a reflect value from the decoder.
func (c *varintcodec) decodeTo(d *decoder, rv reflect.Value) (err error) {
	var v int64
	if v, err = d.readVarint(); err != nil {
		return err
	}
	rv.SetInt(v)
	return err
}

// ------------------------------------------------------------------------------

type varuintcodec struct{}

// Encode encodes a value into the encoder.
func (c *varuintcodec) encodeTo(e *encoder, rv reflect.Value) (err error) {
	uintVal := rv.Uint()
	e.writeUvarint(uintVal)
	return e.err
}

// Decode decodes into a reflect value from the decoder.
func (c *varuintcodec) decodeTo(d *decoder, rv reflect.Value) (err error) {
	var v uint64
	if v, err = d.readUvarint(); err != nil {
		return err
	}
	rv.SetUint(v)
	return err
}

// ------------------------------------------------------------------------------

type float32codec struct{}

// Encode encodes a value into the encoder.
func (c *float32codec) encodeTo(e *encoder, rv reflect.Value) (err error) {
	floatVal := rv.Float()
	e.writeFloat32(float32(floatVal))
	return e.err
}

// Decode decodes into a reflect value from the decoder.
func (c *float32codec) decodeTo(d *decoder, rv reflect.Value) (err error) {
	var v float32
	if v, err = d.readFloat32(); err == nil {
		rv.SetFloat(float64(v))
	}
	return err
}

// ------------------------------------------------------------------------------

type float64codec struct{}

// Encode encodes a value into the encoder.
func (c *float64codec) encodeTo(e *encoder, rv reflect.Value) (err error) {
	floatVal := rv.Float()
	e.writeFloat64(floatVal)
	return e.err
}

// Decode decodes into a reflect value from the decoder.
func (c *float64codec) decodeTo(d *decoder, rv reflect.Value) (err error) {
	var v float64
	if v, err = d.readFloat64(); err == nil {
		rv.SetFloat(v)
	}
	return err
}

// ------------------------------------------------------------------------------

type mapcodec struct {
	keycodec   codec
	valuecodec codec
}

// Encode encodes a value into the encoder.
func (c *mapcodec) encodeTo(e *encoder, rv reflect.Value) (err error) {
	l := rv.Len()
	e.writeUvarint(uint64(l))
	iter := rv.MapRange()
	for iter.Next() {
		if err = c.keycodec.encodeTo(e, iter.Key()); err != nil {
			return err
		}
		if err = c.valuecodec.encodeTo(e, iter.Value()); err != nil {
			return err
		}
	}
	return e.err
}

// Decode decodes into a reflect value from the decoder.
func (c *mapcodec) decodeTo(d *decoder, rv reflect.Value) (err error) {
	var l uint64
	if l, err = d.readUvarint(); err == nil {
		typ := rv.Type()
		newMap := reflect.MakeMapWithSize(typ, int(l))
		keyTyp := typ.Key()
		valTyp := typ.Elem()
		for i := 0; i < int(l); i++ {
			newKey := reflect.New(keyTyp).Elem()
			if err = c.keycodec.decodeTo(d, newKey); err != nil {
				return err
			}
			newVal := reflect.New(valTyp).Elem()
			if err = c.valuecodec.decodeTo(d, newVal); err != nil {
				return err
			}
			newMap.SetMapIndex(newKey, newVal)
		}
		rv.Set(newMap)
	}
	return err
}
