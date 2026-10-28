package awml

import (
	"fmt"
	"testing"
)

func TestProbe(t *testing.T) {
	for _, s := range []string{"# a\n# b\nawml: x\n", "# a\n\n# b\nawml: x\n", "# a\n# b\n\nawml: x # c\n", "\n#a\nawml: x\n", "#  indented\n#\n# - item\nawml: x\n"} {
		c, _ := Comments([]byte(s))
		fmt.Printf("%q\n", c)
	}
}
