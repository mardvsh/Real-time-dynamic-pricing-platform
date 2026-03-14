package internal

import "testing"

func TestValidEvent(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name  string
		event string
		want  bool
	}{
		{name: "view ok", event: "view_product", want: true},
		{name: "cart ok", event: "add_to_cart", want: true},
		{name: "purchase ok", event: "purchase", want: true},
		{name: "invalid", event: "something_else", want: false},
	}

	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			if got := validEvent(tc.event); got != tc.want {
				t.Fatalf("validEvent(%q)=%v, want %v", tc.event, got, tc.want)
			}
		})
	}
}
