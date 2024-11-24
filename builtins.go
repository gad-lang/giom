package gber

import (
	"strconv"
	"strings"

	"github.com/gad-lang/gad"
)

var (
	BuiltinEscape = &gad.Function{
		Name: "escape",
		Value: func(call gad.Call) (_ gad.Object, err error) {
			if err = call.Args.CheckLen(1); err != nil {
				return
			}

			var (
				value = call.Args.GetOnly(0)
				s     string
			)

			switch t := value.(type) {
			case gad.RawStr:
				s = string(t)
			case gad.Str:
				s = string(t)
			default:
				var v gad.Object
				if v, err = call.VM.Builtins.Call(gad.BuiltinStr, call); err != nil {
					return
				}
				s = string(v.(gad.Str))
			}

			return gad.RawStr(s), nil
		},
	}

	AttrFunc = func(vm *gad.VM, name, value gad.Object) (ret gad.RawStr, err error) {
		var (
			toRawStr = vm.Builtins.ArgsInvoker(gad.BuiltinRawStr, gad.Call{VM: vm})
		)

		if value.IsFalsy() {
			return
		}

		if _, ok := name.(gad.RawStr); !ok {
			if name, err = toRawStr(name); err != nil {
				return
			}
		}

		switch t := value.(type) {
		case gad.Array:
			var b strings.Builder
			for _, o := range t {
				if o.IsFalsy() {
					continue
				}
				if _, ok := o.(gad.RawStr); !ok {
					if o, err = toRawStr(o); err != nil {
						return
					}
				}
				b.WriteString(string(o.(gad.RawStr)))
				b.WriteString(" ")
			}
			value = gad.RawStr(strings.TrimSpace(b.String()))
		case gad.RawStr:
		case gad.Flag:
			if t {
				return gad.RawStr(name.ToString()), nil
			}
			return "", nil
		default:
			if value, err = toRawStr(value); err != nil {
				return
			}
		}
		return gad.RawStr(name.ToString() + "=" + strconv.Quote(value.ToString())), nil
	}

	BuiltinAttr = &gad.Function{
		Name: "attr",
		Value: func(call gad.Call) (ret gad.Object, err error) {
			if err = call.Args.CheckLen(2); err != nil {
				return
			}

			ret, err = AttrFunc(call.VM, call.Args.GetOnly(0), call.Args.GetOnly(1))
			return
		},
	}

	BuiltinAttrs = &gad.Function{
		Name: "attrs",
		Value: func(call gad.Call) (_ gad.Object, err error) {
			var (
				b  strings.Builder
				rs gad.RawStr
			)

			call.NamedArgs.Walk(func(na *gad.KeyValue) error {
				if rs, err = AttrFunc(call.VM, na.K, na.V); err == nil && rs != "" {
					b.WriteString(" " + string(rs))
				}
				return err
			})

			if err != nil {
				return
			}

			return gad.RawStr(b.String()), nil
		},
	}
)

func AppendBuiltins(b *gad.Builtins) *gad.Builtins {
	b.Set(BuiltinEscape.Name, BuiltinEscape)
	b.Set(BuiltinAttr.Name, BuiltinAttr)
	b.Set(BuiltinAttrs.Name, BuiltinAttrs)
	return b
}
