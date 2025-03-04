package idproto

import (
	"fmt"
	"strconv"

	"github.com/opencontainers/go-digest"
	"github.com/vektah/gqlparser/v2/ast"
)

type Literate interface {
	ToLiteral() *Literal
}

func (lit *Literal) Modules() []*Module {
	switch v := lit.Value.(type) {
	case *Literal_Id:
		return v.Id.Modules()
	case *Literal_List:
		mods := []*Module{}
		for _, val := range v.List.Values {
			mods = append(mods, val.Modules()...)
		}
		return mods
	case *Literal_Object:
		mods := []*Module{}
		for _, arg := range v.Object.Values {
			mods = append(mods, arg.Value.Modules()...)
		}
		return mods
	default:
		return nil
	}
}

// Canonical returns the literal with any contained IDs canonicalized.
func (lit *Literal) Canonical() *Literal {
	switch v := lit.Value.(type) {
	case *Literal_Id:
		return &Literal{Value: &Literal_Id{Id: v.Id.Canonical()}}
	case *Literal_List:
		list := make([]*Literal, len(v.List.Values))
		for i, val := range v.List.Values {
			list[i] = val.Canonical()
		}
		return &Literal{Value: &Literal_List{List: &List{Values: list}}}
	case *Literal_Object:
		args := make([]*Argument, len(v.Object.Values))
		for i, arg := range v.Object.Values {
			args[i] = arg.Canonical()
		}
		return &Literal{Value: &Literal_Object{Object: &Object{Values: args}}}
	default:
		return lit
	}
}

func (lit *Literal) Tainted() bool {
	switch v := lit.Value.(type) {
	case *Literal_Id:
		return v.Id.IsTainted()
	case *Literal_List:
		for _, val := range v.List.Values {
			if val.Tainted() {
				return true
			}
		}
		return false
	case *Literal_Object:
		for _, arg := range v.Object.Values {
			if arg.Tainted() {
				return true
			}
		}
		return false
	default:
		return false
	}
}

// ToAST returns an AST value appropriate for passing to a GraphQL server.
func (lit *Literal) Display() string {
	switch x := lit.GetValue().(type) {
	case *Literal_Id:
		return fmt.Sprintf("{%s}", x.Id.Display())
	case *Literal_Null:
		return "null"
	case *Literal_Bool:
		return strconv.FormatBool(x.Bool)
	case *Literal_Enum:
		return x.Enum
	case *Literal_Int:
		return fmt.Sprintf("%d", x.Int)
	case *Literal_Float:
		return strconv.FormatFloat(x.Float, 'f', -1, 64)
	case *Literal_String_:
		return truncate(strconv.Quote(x.String_), 100)
	case *Literal_List:
		list := "["
		for i, val := range x.List.Values {
			if i > 0 {
				list += ","
			}
			list += val.Display()
		}
		list += "]"
		return list
	case *Literal_Object:
		obj := "{"
		for i, field := range x.Object.Values {
			if i > 0 {
				obj += ","
			}
			obj += field.Name + ": " + field.Value.Display()
		}
		obj += "}"
		return obj
	default:
		panic(fmt.Sprintf("unsupported literal type %T", x))
	}
}

// ToInput returns a value appropriate for passing to an InputDecoder with
// minimal encoding/decoding overhead.
func (lit *Literal) ToInput() any {
	switch x := lit.GetValue().(type) {
	case *Literal_Id:
		return x.Id
	case *Literal_Null: // TODO remove?
		return nil
	case *Literal_Bool:
		return x.Bool
	case *Literal_Enum:
		return x.Enum
	case *Literal_Int:
		return x.Int
	case *Literal_Float:
		return x.Float
	case *Literal_String_:
		return x.String_
	case *Literal_List:
		list := make([]any, len(x.List.Values))
		for i, val := range x.List.Values {
			list[i] = val.ToInput()
		}
		return list
	case *Literal_Object:
		obj := make(map[string]any, len(x.Object.Values))
		for _, field := range x.Object.Values {
			obj[field.Name] = field.Value.ToInput()
		}
		return obj
	default:
		panic(fmt.Sprintf("unsupported literal type %T", x))
	}
}

// ToAST returns an AST value appropriate for passing to a GraphQL server.
func (lit *Literal) ToAST() *ast.Value {
	switch x := lit.GetValue().(type) {
	case *Literal_Id:
		enc, err := x.Id.Encode()
		if err != nil {
			panic(err)
		}
		return &ast.Value{
			Raw:  enc,
			Kind: ast.StringValue,
		}
	case *Literal_Null:
		return &ast.Value{
			Raw:  "null",
			Kind: ast.NullValue,
		}
	case *Literal_Bool:
		return &ast.Value{
			Raw:  strconv.FormatBool(x.Bool),
			Kind: ast.BooleanValue,
		}
	case *Literal_Enum:
		return &ast.Value{
			Raw:  x.Enum,
			Kind: ast.EnumValue,
		}
	case *Literal_Int:
		return &ast.Value{
			Raw:  fmt.Sprintf("%d", x.Int),
			Kind: ast.IntValue,
		}
	case *Literal_Float:
		return &ast.Value{
			Raw:  strconv.FormatFloat(x.Float, 'f', -1, 64),
			Kind: ast.FloatValue,
		}
	case *Literal_String_:
		return &ast.Value{
			Raw:  x.String_,
			Kind: ast.StringValue,
		}
	case *Literal_List:
		list := &ast.Value{
			Kind: ast.ListValue,
		}
		for _, val := range x.List.Values {
			list.Children = append(list.Children, &ast.ChildValue{
				Value: val.ToAST(),
			})
		}
		return list
	case *Literal_Object:
		obj := &ast.Value{
			Kind: ast.ObjectValue,
		}
		for _, field := range x.Object.Values {
			obj.Children = append(obj.Children, &ast.ChildValue{
				Name:  field.Name,
				Value: field.Value.ToAST(),
			})
		}
		return obj
	default:
		panic(fmt.Sprintf("unsupported literal type %T", x))
	}
}

func truncate(s string, length int) string {
	if len(s) <= length {
		return s
	}

	if length < 5 {
		return s[:length]
	}

	dig := digest.FromString(s)
	prefixLength := (length - 3) / 2
	suffixLength := length - 3 - prefixLength
	abbrev := s[:prefixLength] + "..." + s[len(s)-suffixLength:]
	return fmt.Sprintf("%s:%d:%s", dig, len(s), abbrev)
}
