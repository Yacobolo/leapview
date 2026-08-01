package app

import "testing"

func TestProtectedPublishingTargetPolicy(t *testing.T) {
	for _, test := range []struct {
		name       string
		production bool
		evaluation bool
		want       bool
	}{
		{
			name:       "enterprise production target",
			production: true,
			want:       true,
		},
		{
			name: "development target",
		},
		{
			name:       "disposable local evaluation target",
			production: true,
			evaluation: true,
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			if got := protectedPublishingTarget(
				test.production,
				test.evaluation,
			); got != test.want {
				t.Fatalf("protected publishing = %t, want %t", got, test.want)
			}
		})
	}
}
