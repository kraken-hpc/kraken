package ipmi

import (
	"encoding/binary"
	"fmt"
	"reflect"
	"strconv"
	"strings"
)

type Packer struct {
	ByteOrder binary.ByteOrder
}

func (p Packer) parseArgs(args string) map[string]string {
	r := make(map[string]string)
	argv := strings.Split(args, ",")
	for _, arg := range argv {
		pair := strings.SplitN(arg, "=", 2)
		if len(pair) == 2 {
			r[strings.TrimSpace(pair[0])] = strings.TrimSpace(pair[1])
		} else {
			r[strings.TrimSpace(pair[0])] = ""
		}
	}
	return r
}

func (p Packer) Cksum2(buf []byte) uint8 {
	i := 0
	for _, b := range buf {
		i = (i + int(b)) % 256
	}
	i = -i
	return uint8(i)
}

func (p Packer) Pack(packet interface{}) (b []byte, e []error) {
	sv := reflect.Indirect(reflect.ValueOf(packet))
	st := sv.Type()
	if st.Kind() != reflect.Struct {
		e = append(e, fmt.Errorf("not a struct: %v", st))
		return
	}
	last := 0
	buf := make([]byte, 1500) // be smarter?
	for i := 0; i < st.NumField(); i++ {
		ft := st.Field(i)
		fv := sv.Field(i)
		flagStr, ok := ft.Tag.Lookup("pack")
		if !ok {
			continue
		}
		flags := p.parseArgs(flagStr)

		switch ft.Type.Kind() {
		case reflect.Array:
			if ft.Type.Elem().Kind() != reflect.Uint8 {
				e = append(e, fmt.Errorf("arrays must by of bytes"))
				continue
			}
			len := ft.Type.Len()
			if _, ok := flags["zeros"]; !ok { // don't copy zeros
				//copy(buf[last:], fv.Bytes())
				reflect.Copy(reflect.ValueOf(buf[last:]), fv)
			}
			last += len
		case reflect.Slice:
			if ft.Type.Elem().Kind() != reflect.Uint8 {
				e = append(e, fmt.Errorf("arrays must by of bytes"))
				continue
			}
			len := fv.Len()
			if _, ok := flags["zeros"]; !ok { // don't copy zeros
				copy(buf[last:], fv.Bytes())
			}
			last += len
		case reflect.Uint8:
			if _, ok := flags["cksum2"]; ok { // are we supposed to fill out a checksum?
				fv.Set(reflect.ValueOf(p.Cksum2(buf[0:last])))
			}
			if ref, ok := flags["len"]; ok { // are we supposed to fill out a length?
				refv := sv.FieldByName(ref)
				if refv.IsValid() && !refv.IsNil() {
					if refv.Kind() == reflect.Array || refv.Kind() == reflect.Slice {
						var len, size, bytes uint8
						len = uint8(refv.Len())
						size = uint8(refv.Type().Elem().Size())
						bytes = len * size
						fv.Set(reflect.ValueOf(bytes))
					}
				}
			}
			buf[last] = uint8(fv.Uint())
			last++
		case reflect.Uint16:
			p.ByteOrder.PutUint16(buf[last:], uint16(fv.Uint()))
			last += 2
		case reflect.Uint32:
			p.ByteOrder.PutUint32(buf[last:], uint32(fv.Uint()))
			last += 4
		case reflect.Uint64:
			p.ByteOrder.PutUint64(buf[last:], uint64(fv.Uint()))
			last += 8
		default:
			e = append(e, fmt.Errorf("unhandled kind: %v", ft.Type.Kind()))
		}
	}
	b = make([]byte, last)
	copy(b, buf)
	return
}

func (p Packer) Unpack(b []byte, packet interface{}) (e []error) {
	sv := reflect.Indirect(reflect.ValueOf(packet))
	st := sv.Type()
	if st.Kind() != reflect.Struct {
		e = append(e, fmt.Errorf("not a struct: %v", st))
		return
	}
	last := 0

	// first we calculate the length we need
	for i := 0; i < st.NumField(); i++ {
		ft := st.Field(i)
		fv := sv.Field(i)
		flagStr, ok := ft.Tag.Lookup("pack")
		if !ok {
			continue
		}
		flags := p.parseArgs(flagStr)

		switch ft.Type.Kind() {
		case reflect.Array:
			if ft.Type.Elem().Kind() != reflect.Uint8 {
				e = append(e, fmt.Errorf("arrays must by of bytes"))
				continue
			}
			len := ft.Type.Len()
			// we have to check to make sure this isn't unexported
			if _, ok := flags["zeros"]; !ok && fv.CanSet() {
				//fv.SetBytes(b[last : last+len])
				reflect.Copy(fv, reflect.ValueOf(b[last:last+len]))
			}
			last += len
		case reflect.Slice:
			if ft.Type.Elem().Kind() != reflect.Uint8 {
				e = append(e, fmt.Errorf("arrays must by of bytes"))
				continue
			}
			// compute len
			len := len(b[last:])
			offStr, ok := flags["fill"]
			if ok {
				off, err := strconv.Atoi(offStr)
				if err != nil {
					e = append(e, err)
					continue
				}
				len += off
			}
			aclen, ok := flags["authcodelen"]
			if ok {
				ac := sv.FieldByName(aclen)
				if ac.Kind() != reflect.Uint8 {
					e = append(e, fmt.Errorf("called authcodelen on invalid type"))
					continue
				}
				if uint8(ac.Uint()) == IPMIAuthTypeNONE {
					len = 0
				} else {
					len = 16
				}
			}
			if _, ok := flags["zeros"]; !ok && len != 0 && fv.CanSet() {
				fv.SetBytes(b[last : last+len])
			}
			last += len
		case reflect.Uint8:
			if _, ok := flags["cksum2"]; ok {
				ck := p.Cksum2(b[0:last])
				if ck != b[last] {
					//e = append(e, fmt.Errorf("checksum mismatch: %x != %x", ck, b[last]))
					fmt.Printf("WARNING: checksum mismatch: %x != %x\n", ck, b[last])
				}
			}
			if _, ok := flags["zeros"]; !ok && fv.CanSet() {
				fv.Set(reflect.ValueOf(uint8(b[last])))
			}
			last++
		case reflect.Uint16:
			if _, ok := flags["zeros"]; !ok && fv.CanSet() {
				fv.Set(reflect.ValueOf(p.ByteOrder.Uint16(b[last:])))
			}
			last += 2
		case reflect.Uint32:
			if _, ok := flags["zeros"]; !ok && fv.CanSet() {
				fv.Set(reflect.ValueOf(p.ByteOrder.Uint32(b[last:])))
			}
			last += 4
		case reflect.Uint64:
			if _, ok := flags["zeros"]; !ok && fv.CanSet() {
				fv.Set(reflect.ValueOf(p.ByteOrder.Uint64(b[last:])))
			}
			last += 8
		default:
			e = append(e, fmt.Errorf("unhandled kind: %v", ft.Type.Kind()))
		}
	}
	return
}

func (p Packer) PackMust(i interface{}) []byte {
	b, es := p.Pack(i)
	if len(es) > 0 {
		fmt.Printf("%v\n", es)
	}
	return b
}
