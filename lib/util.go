/* util.go: provides useful methods used throughout Kraken
 *
 * Author: J. Lowell Wofford <lowell@lanl.gov>
 *
 * This software is open source software available under the BSD-3 license.
 * Copyright (c) 2018, Triad National Security, LLC
 * See LICENSE file for details.
 */

package lib

import (
	fmt "fmt"
	"log"
	"reflect"
	"strconv"
	"strings"

	proto "github.com/golang/protobuf/proto"
	"github.com/golang/protobuf/ptypes"
	"github.com/golang/protobuf/ptypes/any"
)

// MessageDiff compares a & b, returns a slice of differing field names
//             returns an error if types are different.
//			   This borrows a lot from the proto.Merge code
func MessageDiff(a, b proto.Message, pre string) (r []string, e error) {
	/*
	 * Sanity checks
	 */
	va := reflect.ValueOf(a)
	vb := reflect.ValueOf(b)
	ta := reflect.TypeOf(a)
	tb := reflect.TypeOf(b)
	// Is either object nil?
	if va.IsNil() || vb.IsNil() {
		e = fmt.Errorf("cannot diff a nil")
		return
	}
	if ta != tb {
		e = fmt.Errorf("refusing to diff objects of different type")
		return
	}
	// Are they equal?
	if proto.Equal(a, b) {
		return
	}
	return diffStruct(va.Elem(), vb.Elem(), pre)
}

func diffStruct(a, b reflect.Value, pre string) (r []string, e error) {
	if !a.IsValid() || !b.IsValid() {
		e = fmt.Errorf("diffStruct called on invalid value(s)")
		return
	}
	if a.Type() != b.Type() {
		e = fmt.Errorf("diffStruct called on mismatched values types: %s vs %s", a.Type(), b.Type())
		return
	}
	for i := 0; i < a.NumField(); i++ {
		f := a.Type().Field(i)
		if f.Name == "Extensions" || f.Name == "Services" || f.Name == "Children" || f.Name == "Parents" || strings.HasPrefix(f.Name, "XXX_") {
			continue
		}
		s, e := diffAny(a.Field(i), b.Field(i), URLPush(pre, f.Name))
		if e != nil {
			return r, e
		}
		r = append(r, s...)
	}
	return
}

func diffSlice(a, b reflect.Value, pre string) (r []string, e error) {
	alen := a.Len()
	blen := b.Len()
	// pad our slices to equal length
	if alen > blen {
		pad := reflect.MakeSlice(a.Type(), alen-blen, alen-blen)
		b = reflect.AppendSlice(b, pad)
	} else if blen > alen {
		pad := reflect.MakeSlice(a.Type(), blen-alen, blen-alen)
		a = reflect.AppendSlice(a, pad)
	}
	for i := 0; i < a.Len(); i++ {
		s, e := diffAny(a.Index(i), b.Index(i), URLPush(pre, strconv.Itoa(i)))
		if e != nil {
			return r, e
		}
		r = append(r, s...)
	}
	return
}

func diffAny(a, b reflect.Value, pre string) (r []string, e error) {
	// TODO: we'll deal with this case later
	/*
		if a.Type() == reflect.TypeOf((*proto.Message)(nil)).Elem() {
		}
	*/
	switch a.Kind() {
	case reflect.Bool, reflect.Float32, reflect.Float64, reflect.Int32, reflect.Int64,
		reflect.String, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
		if a.Interface() != b.Interface() {
			return []string{pre}, nil
		}
	case reflect.Struct:
		return diffStruct(a, b, pre)
	case reflect.Slice:
		return diffSlice(a, b, pre)
	case reflect.Ptr:
		return diffAny(a.Elem(), b.Elem(), pre)
	case reflect.Interface, reflect.Map:
		fallthrough
	default:
		// log.Printf("not yet implemented: %v", a.Kind())
		// log.Printf("unknown type")
	}
	return
}

// ResolveURL turns a state URL (string) into an interface reference
func ResolveURL(url string, context reflect.Value) (v reflect.Value, e error) {
	if url == "" {
		v = context
		return
	}

	root, sub := URLShift(url)

	switch context.Kind() {
	case reflect.Map:
		log.Println("map resolution is not yet implemented")
	case reflect.Ptr: // resolve pointers to their target
		v, e = ResolveURL(url, context.Elem())
	case reflect.Slice: // root should be a slice index
		i, err := strconv.Atoi(root)
		if err != nil {
			e = fmt.Errorf("non-integer child of slice: %s", root)
			return
		}
		if context.Len()-1 < i {
			e = fmt.Errorf("no such slice index: %d", i)
			return
		}
		v, e = ResolveURL(sub, context.Index(i))
	case reflect.Struct: // root should be a field name
		if context.Type() == reflect.TypeOf(any.Any{}) {
			any := context.Interface().(any.Any)
			var da ptypes.DynamicAny
			if err := ptypes.UnmarshalAny(&any, &da); err != nil {
				e = fmt.Errorf("failed to unmarshal any.Any object, %v", err)
				return
			}
			v, e = ResolveURL(root, reflect.ValueOf(da.Message).Elem())
		} else if f := context.FieldByName(root); !f.IsValid() {
			e = fmt.Errorf("field not found in struct, %s", root)
		} else {
			v, e = ResolveURL(sub, f)
		}
	default:
		e = fmt.Errorf("cannot resolve property, %s, in simple type, %s", root, context.Kind().String())
	}
	return
}

// ResolveOrMakeURL is like ResolveURL, except it will create any referenced but non-defined objects
func ResolveOrMakeURL(url string, context reflect.Value) (v reflect.Value, e error) {
	if url == "" {
		v = context
		return
	}

	root, sub := URLShift(url)

	switch context.Kind() {
	case reflect.Map:
		log.Println("map resolution is not yet implemented")
	case reflect.Ptr: // resolve pointers to their target
		if !context.Elem().IsValid() {
			// we need to allocate an object here
			new := reflect.New(context.Type().Elem())
			context.Set(new)
		}
		v, e = ResolveOrMakeURL(url, context.Elem())
	case reflect.Slice: // root should be a slice index
		i, err := strconv.Atoi(root)
		if err != nil {
			e = fmt.Errorf("non-integer child of slice: %s", root)
			return
		}
		if context.Len()-1 < i {
			// grow the slice as needed
			len := i + 1 - context.Len()
			t := context.Type()
			a := reflect.MakeSlice(t, len, len)
			context.Set(reflect.AppendSlice(context, a))
		}
		v, e = ResolveOrMakeURL(sub, context.Index(i))
	case reflect.Struct: // root should be a field name
		if f := context.FieldByName(root); !f.IsValid() {
			e = fmt.Errorf("field not found in struct, %s", root)
		} else {
			v, e = ResolveOrMakeURL(sub, f)
		}
	default:
		e = fmt.Errorf("cannot resolve property, %s, in simple type, %s", root, context.Kind().String())
	}
	return
}

// URLShift gives the current root of a url and the remaining url
func URLShift(url string) (root string, sub string) {
	ret := strings.SplitN(url, "/", 2)
	switch len(ret) {
	case 0:
		root = ""
		sub = ""
	case 1:
		root = url
		sub = ""
	case 2:
		if ret[0] == "" { // beginning slash
			ret[0], ret[1] = URLShift(ret[1])
		}
		root = ret[0]
		sub = ret[1]
	}
	return
}

func URLPush(url string, elem string) string {
	return url + "/" + elem
}

func URLToSlice(url string) (s []string) {
	s = strings.Split(url, "/")
	return
}

func SliceToURL(s []string) (url string) {
	url = strings.Join(s, "/")
	return
}

func NodeURLSplit(s string) (node string, url string) {
	ret := strings.SplitN(s, ":", 2)
	switch len(ret) {
	case 0: // empty string
		node = ""
		url = ""
	case 1: // we have just a node?
		node = ret[0]
		url = ""
	case 2:
		node = ret[0]
		url = ret[1]
		break
	}
	return
}

func NodeURLJoin(node, url string) string {
	return node + ":" + url
}

/*
type GenericProtoMessage struct {
	m proto.Message
}

func (g *GenericProtoMessage) GetMessage() proto.Message  { return g.m }
func (g *GenericProtoMessage) SetMessage(m proto.Message) { g.m = m }
func (g *GenericProtoMessage) MarshalProto() ([]byte, error) {
	return proto.Marshal(g.m)
}
func (g *GenericProtoMessage) UnmarshalProto(b []byte) error {
	return proto.Unmarshal(b, g.m)
}
func (g *GenericProtoMessage) MarshalJSON() ([]byte, error) {
	return MarshalJSON(g)
}
func (g *GenericProtoMessage) UnmarshalJSON(b []byte) error {
	return UnmarshalJSON(b, g.m)
}
*/

// ValueToString does its best to convert values into sensible strings for printing
func ValueToString(v reflect.Value) (s string) {
	switch v.Kind() {
	case reflect.String:
		s = v.String()
	case reflect.Uint:
		s = fmt.Sprintf("%d", v.Uint())
	case reflect.Int:
		s = fmt.Sprintf("%d", v.Int())
	case reflect.Bool:
		s = fmt.Sprintf("%t", v.Bool())
	case reflect.Struct:
		s = fmt.Sprintf("%v", v.Interface())
	default:
		s = fmt.Sprintf("%v", v)
	}
	return
}
