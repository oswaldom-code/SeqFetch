package pattern_test

import (
	"testing"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

func TestPattern(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Pattern Suite")
}
