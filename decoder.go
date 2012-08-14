package thrift

import (
	"io"
	"reflect"
	"runtime"
)

type Decoder interface {
	DecodeThrift(io.Reader, Protocol) error
}

type decoder struct {
	r io.Reader
	p Protocol
}

func DecodeStruct(r io.Reader, protocol Protocol, v interface{}) (err error) {
	if de, ok := v.(Decoder); ok {
		return de.DecodeThrift(r, protocol)
	}

	defer func() {
		if r := recover(); r != nil {
			if _, ok := r.(runtime.Error); ok {
				panic(r)
			}
			err = r.(error)
		}
	}()
	d := &decoder{r, protocol}
	vo := reflect.ValueOf(v)
	for vo.Kind() != reflect.Ptr {
		d.error(&UnsupportedValueError{Value: vo, Str: "pointer to struct expected"})
	}
	if vo.Elem().Kind() != reflect.Struct {
		d.error(&UnsupportedValueError{Value: vo, Str: "expected a struct"})
	}
	d.readValue(typeStruct, vo.Elem())
	return nil
}

func (d *decoder) error(err interface{}) {
	panic(err)
}

func (d *decoder) readValue(thriftType byte, rf reflect.Value) {
	v := rf
	kind := rf.Kind()
	if kind == reflect.Ptr {
		if rf.IsNil() {
			rf.Set(reflect.New(rf.Type().Elem()))
		}
		v = rf.Elem()
		kind = v.Kind()
	}

	var err error = nil
	switch thriftType {
	case typeBool:
		if val, err := d.p.ReadBool(d.r); err != nil {
			d.error(err)
		} else {
			v.SetBool(val)
		}
	case typeByte:
		if val, err := d.p.ReadByte(d.r); err != nil {
			d.error(err)
		} else {
			v.SetInt(int64(val))
		}
	case typeI16:
		if val, err := d.p.ReadI16(d.r); err != nil {
			d.error(err)
		} else {
			v.SetInt(int64(val))
		}
	case typeI32:
		if val, err := d.p.ReadI32(d.r); err != nil {
			d.error(err)
		} else {
			v.SetInt(int64(val))
		}
	case typeI64:
		if val, err := d.p.ReadI64(d.r); err != nil {
			d.error(err)
		} else {
			v.SetInt(val)
		}
	case typeDouble:
		if val, err := d.p.ReadDouble(d.r); err != nil {
			d.error(err)
		} else {
			v.SetFloat(val)
		}
	case typeString:
		if kind == reflect.Slice {
			elemType := v.Type().Elem()
			if elemType.Kind() == reflect.Uint8 && elemType.Name() == "byte" {
				if val, err := d.p.ReadBytes(d.r); err != nil {
					d.error(err)
				} else {
					v.SetBytes(val)
				}
			} else {
				err = &UnsupportedValueError{Value: v, Str: "expected a byte array"}
			}
		} else {
			if val, err := d.p.ReadString(d.r); err != nil {
				d.error(err)
			} else {
				v.SetString(val)
			}
		}
	case typeStruct:
		if err := d.p.ReadStructBegin(d.r); err != nil {
			d.error(err)
		}

		meta := encodeFields(v.Type())
		req := meta.required
		for {
			ftype, id, err := d.p.ReadFieldBegin(d.r)
			if err != nil {
				d.error(err)
			}
			if ftype == typeStop {
				break
			}

			ef, ok := meta.fields[int(id)]
			if !ok {
				SkipValue(d.r, d.p, ftype)
			} else {
				req &= ^(uint64(1) << uint64(id))
				fieldValue := v.Field(ef.i)
				if ftype != ef.fieldType {
					d.error(&UnsupportedValueError{Value: fieldValue, Str: "type mismatch"})
				}
				d.readValue(ftype, fieldValue)
			}

			if err = d.p.ReadFieldEnd(d.r); err != nil {
				d.error(err)
			}
		}

		if err := d.p.ReadStructEnd(d.r); err != nil {
			d.error(err)
		}

		if req != 0 {
			for i := 0; req != 0; i, req = i+1, req>>1 {
				if req&1 != 0 {
					d.error(&MissingRequiredField{
						StructName: v.Type().Name(),
						FieldName:  meta.fields[i].name,
					})
				}
			}
		}
	case typeMap:
		keyType := v.Type().Key()
		valueType := v.Type().Elem()
		ktype, vtype, n, err := d.p.ReadMapBegin(d.r)
		if err != nil {
			d.error(err)
		}
		v.Set(reflect.MakeMap(v.Type()))
		for i := 0; i < n; i++ {
			key := reflect.New(keyType).Elem()
			val := reflect.New(valueType).Elem()
			d.readValue(ktype, key)
			d.readValue(vtype, val)
			v.SetMapIndex(key, val)
		}
		if err := d.p.ReadMapEnd(d.r); err != nil {
			d.error(err)
		}
	case typeList:
		elemType := v.Type().Elem()
		et, n, err := d.p.ReadListBegin(d.r)
		if err != nil {
			d.error(err)
		}
		for i := 0; i < n; i++ {
			val := reflect.New(elemType)
			d.readValue(et, val.Elem())
			v.Set(reflect.Append(v, val.Elem()))
		}
		if err := d.p.ReadListEnd(d.r); err != nil {
			d.error(err)
		}
	case typeSet:
		elemType := v.Type().Elem()
		et, n, err := d.p.ReadSetBegin(d.r)
		if err != nil {
			d.error(err)
		}
		for i := 0; i < n; i++ {
			val := reflect.New(elemType)
			d.readValue(et, val.Elem())
			v.Set(reflect.Append(v, val.Elem()))
		}
		if err := d.p.ReadSetEnd(d.r); err != nil {
			d.error(err)
		}
	default:
		d.error(&UnsupportedTypeError{v.Type()})
	}

	if err != nil {
		d.error(err)
	}

	return
}
