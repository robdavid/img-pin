package enum

// EnumValue implements the [pflag.Value] interface for enum types satisfying
// [StringableInt].
type EnumValue[T StringableInt] struct {
	value *T
}

// NewEnumValue returns a new EnumValue that stores parsed values into the
// given pointer.
func NewEnumValue[T StringableInt](value *T) *EnumValue[T] {
	return &EnumValue[T]{value: value}
}

func (v *EnumValue[T]) String() string {
	return (*v.value).String()
}

func (v *EnumValue[T]) Set(name string) error {
	result, err := FromString[T](name)
	if err != nil {
		return err
	}
	*v.value = result
	return nil
}

func (v *EnumValue[T]) Type() string {
	//var zero T
	return "string"
	//return fmt.Sprintf("%T", zero)
}
