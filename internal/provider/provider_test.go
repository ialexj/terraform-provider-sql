package provider

import (
	"testing"
	// "github.com/ialexj/terraform-provider-sql/internal/server"
)

func TestServer(t *testing.T) {
	_ = New("acctest")()

	// s.Test(t)
}
