package sfnfunction

import (
	"context"
	"encoding/json"
	"fmt"
	"reflect"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/service/sfn"
	errors "golang.org/x/xerrors"
	"gopkg.in/Clever/kayvee-go.v6/logger"
)

// Resource is an SFN resource that calls out to a function.
type Resource struct {
	name string
	fn   normalizedFn
}

type normalizedFn func(ctx context.Context, input string) (interface{}, error)

// ErrBadFunctionSignature is returned when the supplied function does not
// match one of the following:
// 	func ()
// 	func () error
// 	func (TIn) error
// 	func () (TOut, error)
// 	func (TIn) (TOut, error)
// 	func (context.Context) error
// 	func (context.Context, TIn) error
// 	func (context.Context) (TOut, error)
// 	func (context.Context, TIn) (TOut, error)
//
// Where "TIn" and "TOut" are types compatible with the "encoding/json" standard library.
type ErrBadFunctionSignature struct {
	reason string
}

func (e ErrBadFunctionSignature) Error() string {
	return e.reason
}

func normalizeFunction(fn interface{}) (normalizedFn, error) {
	if fn == nil {
		return nil, errors.New("function is nil")
	}
	fnVal := reflect.ValueOf(fn)
	fnType := reflect.TypeOf(fn)
	if fnType.Kind() != reflect.Func {
		return nil, fmt.Errorf("handler kind %s is not %s", fnType.Kind(), reflect.Func)
	}
	takesContext, err := validateArguments(fnType)
	if err != nil {
		return nil, err
	}
	if err := validateReturns(fnType); err != nil {
		return nil, err
	}

	return normalizedFn(func(ctx context.Context, input string) (interface{}, error) {
		// construct arguments
		var args []reflect.Value
		if takesContext {
			args = append(args, reflect.ValueOf(ctx))
		}
		if (fnType.NumIn() == 1 && !takesContext) || fnType.NumIn() == 2 {
			inputType := fnType.In(fnType.NumIn() - 1)
			inputValue := reflect.New(inputType)

			if err := json.Unmarshal([]byte(input), inputValue.Interface()); err != nil {
				return nil, err
			}
			args = append(args, inputValue.Elem())
		}

		returnValues := fnVal.Call(args)

		// convert return values into (interface{}, error)
		var err error
		if len(returnValues) > 0 {
			if errVal, ok := returnValues[len(returnValues)-1].Interface().(error); ok {
				err = errVal
			}
		}
		var val interface{}
		if len(returnValues) > 1 {
			val = returnValues[0].Interface()
		}

		return val, err
	}), nil
}

func validateArguments(fnType reflect.Type) (bool, error) {
	takesContext := false
	if fnType.NumIn() > 2 {
		return false, fmt.Errorf("function may not take more than two arguments, but function takes %d", fnType.NumIn())
	} else if fnType.NumIn() > 0 {
		contextType := reflect.TypeOf((*context.Context)(nil)).Elem()
		argumentType := fnType.In(0)
		takesContext = argumentType.Implements(contextType)
		if fnType.NumIn() > 1 && !takesContext {
			return false, fmt.Errorf("function takes two arguments, but the first is not Context. got %s", argumentType.Kind())
		}
	}
	return takesContext, nil
}

func validateReturns(fnType reflect.Type) error {
	errorType := reflect.TypeOf((*error)(nil)).Elem()
	if fnType.NumOut() > 2 {
		return fmt.Errorf("function may not return more than two values")
	} else if fnType.NumOut() > 1 {
		if !fnType.Out(1).Implements(errorType) {
			return fmt.Errorf("function returns two values, but the second does not implement error")
		}
	} else if fnType.NumOut() == 1 {
		if !fnType.Out(0).Implements(errorType) {
			return fmt.Errorf("function returns a single value, but it does not implement error")
		}
	}
	return nil
}

// New returns a new function resource.
func New(name string, fn interface{}) (*Resource, error) {
	newFn, err := normalizeFunction(fn)
	if err != nil {
		return nil, ErrBadFunctionSignature{err.Error()}
	}
	return &Resource{name: name, fn: normalizedFn(newFn)}, nil
}

// Result encodes whether the function call should result in calling
// SendTaskSuccess or SendTaskFailure.
type Result struct {
	Success *sfn.SendTaskSuccessInput
	Failure *sfn.SendTaskFailureInput
}

// getType gets the struct name.
func getType(myvar interface{}) string {
	t := reflect.TypeOf(myvar)
	if t.Kind() == reflect.Ptr {
		return t.Elem().Name()
	}
	return t.Name()
}

// Call the function.
func (r *Resource) Call(ctx context.Context, input string) Result {
	logger.FromContext(ctx).AddContext("resource", r.name)
	out, err := r.fn(ctx, input)
	if err != nil {
		var cause string
		if _, ok := err.(errors.Formatter); ok {
			cause = fmt.Sprintf("%+v", err)
		} else {
			cause = err.Error()
		}
		return Result{
			Failure: &sfn.SendTaskFailureInput{
				Error: aws.String(getType(err)),
				Cause: aws.String(cause),
			},
		}
	}
	output := "{}"
	if out != nil {
		bs, err := json.Marshal(out)
		if err != nil {
			return Result{
				Failure: &sfn.SendTaskFailureInput{
					Error: aws.String(getType(err)),
					Cause: aws.String(err.Error()),
				},
			}
		}
		output = string(bs)
	}
	return Result{
		Success: &sfn.SendTaskSuccessInput{
			Output: aws.String(output),
		},
	}
}
