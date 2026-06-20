package logmask

import "testing"

func TestMask(t *testing.T) {
	cases := []struct {
		name string
		in   string
		want string
	}{
		{
			name: "shell style password",
			in:   "user=alice password=hunter2 region=us",
			want: "user=alice password=*** region=us",
		},
		{
			name: "json token field",
			in:   `{"user":"alice","token":"abc.def.ghi","ttl":3600}`,
			want: `{"user":"alice","token":"***","ttl":3600}`,
		},
		{
			name: "http header authorization",
			in:   "Authorization: Bearer eyJ.something.signature",
			want: "Authorization: ***",
		},
		{
			name: "case insensitive PASSWORD",
			in:   "PASSWORD=Hunter2 trace=abc",
			want: "PASSWORD=*** trace=abc",
		},
		{
			name: "non sensitive line passes through",
			in:   "GET /health 200 12ms client=172.19.0.1",
			want: "GET /health 200 12ms client=172.19.0.1",
		},
		{
			name: "json authorization field",
			in:   `{"authorization":"Bearer eyJ"}`,
			want: `{"authorization":"***"}`,
		},
		{
			name: "api_key bare value",
			in:   "api_key=sk-12345 something=else",
			want: "api_key=*** something=else",
		},
		{
			name: "secret in middle",
			in:   "starting handler secret=topsecret thing=ok",
			want: "starting handler secret=*** thing=ok",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := Mask(tc.in)
			if got != tc.want {
				t.Fatalf("Mask(%q)\n got: %q\nwant: %q", tc.in, got, tc.want)
			}
		})
	}
}

func TestMaskEmpty(t *testing.T) {
	if Mask("") != "" {
		t.Fatal("empty in must return empty")
	}
}
