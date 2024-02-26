package liberlogger

import (
	"reflect"
	"testing"
)

type User struct {
	Name     string
	Document string
}

type Card struct {
	Code  int
	Brand string
}

type UserWithCard struct {
	Name     string
	Document string
	Card     Card
}

type UserWithCardPointer struct {
	Name     string
	Document string
	Card     *Card
}

func TestRedact(t *testing.T) {
	value := 12345
	type args struct {
		keysToRedact []string
		keysToMask   []string
		body         interface{}
	}

	tests := []struct {
		name string
		args args
		want interface{}
	}{
		{
			name: "Should not redact a int",
			args: args{
				keysToRedact: []string{"document"},
				body:         1,
			},
			want: 1,
		},
		{
			name: "Should redact a property from a map[string]interface{}",
			args: args{
				keysToRedact: []string{"document"},
				body: map[string]interface{}{
					"name":     "Joao Silva",
					"document": "0192309128390128",
				},
			},
			want: map[string]interface{}{
				"name":     "Joao Silva",
				"document": "REDACTED",
			},
		},
		{
			name: "Should redact a property from a pointer to a map[string]interface{}",
			args: args{
				keysToRedact: []string{"document"},
				body: &map[string]interface{}{
					"name":     "Joao Silva",
					"document": "0192309128390128",
				},
			},
			want: map[string]interface{}{
				"name":     "Joao Silva",
				"document": "REDACTED",
			},
		},
		{
			name: "Should redact a property to a map[string]interface{} with a pointer",
			args: args{
				keysToRedact: []string{"document"},
				body: &map[string]interface{}{
					"name":     "Joao Silva",
					"balance":  &value,
					"document": &value,
				},
			},
			want: map[string]interface{}{
				"name":     "Joao Silva",
				"document": "REDACTED",
				"balance":  12345,
			},
		},
		{
			name: "Should redact a property to a map[string]interface{} with a nested pointer",
			args: args{
				keysToRedact: []string{"document", "code"},
				body: &map[string]interface{}{
					"name": "Joao Silva",
					"card": &map[string]interface{}{
						"code":    123,
						"balance": &value,
					},
					"document": &value,
				},
			},
			want: map[string]interface{}{
				"name":     "Joao Silva",
				"document": "REDACTED",
				"card": map[string]interface{}{
					"code":    "REDACTED",
					"balance": 12345,
				},
			},
		},
		{
			name: "Should mask a property from a map[string]interface{}",
			args: args{
				keysToMask: []string{"document"},
				body: map[string]interface{}{
					"name":     "Joao Silva",
					"document": "58707647000",
				},
			},
			want: map[string]interface{}{
				"name":     "Joao Silva",
				"document": "5870****000",
			},
		},
		{
			name: "Should mask and redact properties from a map[string]interface{}",
			args: args{
				keysToMask:   []string{"document"},
				keysToRedact: []string{"balance"},
				body: map[string]interface{}{
					"name":     "Joao Silva",
					"document": "58707647000",
					"balance":  120938102983,
				},
			},
			want: map[string]interface{}{
				"name":     "Joao Silva",
				"document": "5870****000",
				"balance":  "REDACTED",
			},
		},
		{
			name: "Should redact a nested property from a map[string]interface{}",
			args: args{
				keysToRedact: []string{"code"},
				body: map[string]interface{}{
					"name": "Joao Silva",
					"card": map[string]interface{}{
						"brand": "mastercard",
						"code":  "123",
					},
				},
			},
			want: map[string]interface{}{
				"name": "Joao Silva",
				"card": map[string]interface{}{
					"brand": "mastercard",
					"code":  "REDACTED",
				},
			},
		},
		{
			name: "Should redact a property from a struct",
			args: args{
				keysToRedact: []string{"document"},
				body:         User{Name: "Joao Silva", Document: "1231412312"},
			},
			want: map[string]interface{}{"Name": "Joao Silva", "Document": "REDACTED"},
		},
		{
			name: "Should redact a property from a pointer struct",
			args: args{
				keysToRedact: []string{"document"},
				body:         &User{Name: "Joao Silva", Document: "1231412312"},
			},
			want: map[string]interface{}{"Name": "Joao Silva", "Document": "REDACTED"},
		},
		{
			name: "Should redact a nested property from a struct with pointer",
			args: args{
				keysToRedact: []string{"code", "document"},
				body: UserWithCardPointer{
					Name:     "Joao Silva",
					Document: "1231412312",
					Card: &Card{
						Code:  123,
						Brand: "Master",
					},
				},
			},
			want: map[string]interface{}{"Name": "Joao Silva", "Document": "REDACTED", "Card": map[string]interface{}{
				"Code":  "REDACTED",
				"Brand": "Master",
			}},
		},
		{
			name: "Should redact a nested property from a struct",
			args: args{
				keysToRedact: []string{"code", "document"},
				body: UserWithCard{
					Name:     "Joao Silva",
					Document: "1231412312",
					Card: Card{
						Code:  123,
						Brand: "Master",
					},
				},
			},
			want: map[string]interface{}{"Name": "Joao Silva", "Document": "REDACTED", "Card": map[string]interface{}{
				"Code":  "REDACTED",
				"Brand": "Master",
			}},
		},
		{
			name: "Should redact a property from a map[string]string{}",
			args: args{
				keysToRedact: []string{"code", "document"},
				body:         map[string]string{"Name": "Joao Santos", "code": "123", "document": "0823947219384", "Surname": "Santos"},
			},
			want: map[string]interface{}{"Name": "Joao Santos", "code": "REDACTED", "document": "REDACTED", "Surname": "Santos"},
		},
		{
			name: "Should redact a property from a map[string]int{}",
			args: args{
				keysToRedact: []string{"code", "value"},
				body:         map[string]int{"code": 123, "document": 823947219384, "value": 99},
			},
			want: map[string]interface{}{"code": "REDACTED", "document": 823947219384, "value": "REDACTED"},
		},
		{
			name: "Should redact a property from a map[string]float{}",
			args: args{
				keysToRedact: []string{"value"},
				body:         map[string]float64{"code": 123.23, "document": 823947219384, "value": 99.88},
			},
			want: map[string]interface{}{"code": 123.23, "document": float64(823947219384), "value": "REDACTED"},
		},
		{
			name: "Should redact struct with property nil",
			args: args{
				keysToRedact: []string{"value"},
				body: struct {
					Inner *struct{}
					Value float64
				}{Inner: nil, Value: 99.88},
			},
			want: map[string]interface{}{"Inner": map[string]interface{}{}, "Value": "REDACTED"},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := Redact(tt.args.keysToRedact, tt.args.keysToMask, tt.args.body); !reflect.DeepEqual(got, tt.want) {
				t.Errorf("Redact() = %+v, want %+v", got, tt.want)
			}
		})
	}
}

func Test_maskValue(t *testing.T) {
	type args struct {
		value string
	}
	tests := []struct {
		name string
		args args
		want string
	}{
		{
			name: "Should mask a CPF correctly",
			args: args{
				value: "77903909029",
			},
			want: "7790****029",
		},
		{
			name: "Should mask a Credit Card correctly",
			args: args{
				value: "5111786674841746",
			},
			want: "511178******1746",
		},
		{
			name: "Should mask a small string correctly",
			args: args{
				value: "123",
			},
			want: "1*3",
		},
		{
			name: "Should mask a small string correctly",
			args: args{
				value: "12",
			},
			want: "1*",
		},
		{
			name: "Should mask a small string correctly",
			args: args{
				value: "1",
			},
			want: "1",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := maskValue(tt.args.value); got != tt.want {
				t.Errorf("maskValue() = %v, want %v", got, tt.want)
			}
		})
	}
}
