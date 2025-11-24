package flags

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestNetAddress_Set(t *testing.T) {
	type fields struct {
		Host string
		Port string
	}
	type args struct {
		s string
	}
	type want struct {
		Host string
		Port string
	}
	tests := []struct {
		name    string
		fields  fields
		args    args
		want    want
		wantErr bool
	}{
		{
			name: "set localhost:9090",
			fields: fields{
				Host: "",
				Port: "",
			},
			args: args{s: "localhost:9090"},
			want: want{
				Host: "localhost",
				Port: "9090",
			},
			wantErr: false,
		},
		{
			name: "empty string",
			fields: fields{
				Host: "",
				Port: "",
			},
			args: args{s: ""},
			want: want{
				Host: "",
				Port: "",
			},
			wantErr: false,
		},
		{
			name: "set :9090",
			fields: fields{
				Host: "",
				Port: "",
			},
			args: args{s: ":9090"},
			want: want{
				Host: "localhost",
				Port: "9090",
			},
			wantErr: false,
		},
		{
			name: "throw error 1",
			fields: fields{
				Host: "",
				Port: "",
			},
			args: args{s: "9090"},
			want: want{
				Host: "localhost",
				Port: "9090",
			},
			wantErr: true,
		},
		{
			name: "throw error 2",
			fields: fields{
				Host: "",
				Port: "",
			},
			args: args{s: "some-string"},
			want: want{
				Host: "localhost",
				Port: "9090",
			},
			wantErr: true,
		},
		{
			name: "throw error 3",
			fields: fields{
				Host: "",
				Port: "",
			},
			args: args{s: ":"},
			want: want{
				Host: "localhost",
				Port: "9090",
			},
			wantErr: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			a := &NetAddress{
				Host: tt.fields.Host,
				Port: tt.fields.Port,
			}
			err := a.Set(tt.args.s)
			if tt.wantErr {
				assert.Error(t, err, "Set() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			assert.Equal(t, tt.want.Port, a.Port)
			assert.Equal(t, tt.want.Host, a.Host)
		})
	}
}
