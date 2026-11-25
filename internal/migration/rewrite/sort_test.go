package rewrite

import (
	"strings"
	"testing"
)

func TestSortBlocksByName(t *testing.T) {
	t.Parallel()
	source := []byte(`resource "fastly_service_vcl" "demo" {
  name = "demo"

  # origin c
  backend {
    name    = "c"
    address = "c.example.com"
  }

  condition {
    name      = "z"
    statement = "req.url ~ \"^/z\""
    type      = "REQUEST"
  }

  backend {
    name    = "a"
    address = <<-EOT
      a.example.com
    EOT
  } # trailing

  backend {
    name    = "b"
    address = "b.example.com"
  }
}
`)
	got, err := (SortBlocksByName{ResourceType: "fastly_service_vcl", ResourceName: "demo", Block: "backend"}).apply(source, "main.tf")
	if err != nil {
		t.Fatal(err)
	}
	want := `resource "fastly_service_vcl" "demo" {
  name = "demo"

  backend {
    name    = "a"
    address = <<-EOT
      a.example.com
    EOT
  } # trailing

  condition {
    name      = "z"
    statement = "req.url ~ \"^/z\""
    type      = "REQUEST"
  }

  backend {
    name    = "b"
    address = "b.example.com"
  }

  # origin c
  backend {
    name    = "c"
    address = "c.example.com"
  }
}
`
	if string(got) != want {
		t.Fatalf("unexpected order:\n%s", got)
	}
	again, err := (SortBlocksByName{ResourceType: "fastly_service_vcl", ResourceName: "demo", Block: "backend"}).apply(got, "main.tf")
	if err != nil || string(again) != want {
		t.Fatal("sorting is not idempotent")
	}
}

func TestSortBlocksByNameNames(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct{ body, want string }{
		{"backend {\n    name = var.name\n  }\n  backend {\n    name = \"a\"\n  }", "literal string name"},
		{"backend {\n    name = \"a\"\n  }\n  backend {\n    name = \"a\"\n  }", "duplicate names"},
		{"backend {\n    address = \"x\"\n  }\n  backend {\n    name = \"a\"\n  }", "literal string name"},
		{"backend {\n    name = var.name\n  }", ""},
	} {
		source := []byte("resource \"fastly_service_vcl\" \"demo\" {\n  " + tc.body + "\n}\n")
		_, err := (SortBlocksByName{ResourceType: "fastly_service_vcl", ResourceName: "demo", Block: "backend"}).apply(source, "main.tf")
		if tc.want == "" {
			if err != nil {
				t.Fatalf("single block unexpectedly rejected: %v", err)
			}
			continue
		}
		if err == nil || !strings.Contains(err.Error(), tc.want) {
			t.Fatalf("got %v, want %q", err, tc.want)
		}
	}
}

func TestSortBlocksByNameBlockComments(t *testing.T) {
	t.Parallel()
	source := []byte(`resource "fastly_service_vcl" "demo" {
  /* origin c */
  backend {
    name = "c"
  } /* trailing c */ # and more

  /*
    origin b
  */
  backend {
    name = "b"
  }

  # origin a
  /* primary */
  backend {
    name = "a"
  } /* Primary origin */
}
`)
	got, err := (SortBlocksByName{ResourceType: "fastly_service_vcl", ResourceName: "demo", Block: "backend"}).apply(source, "main.tf")
	if err != nil {
		t.Fatal(err)
	}
	want := `resource "fastly_service_vcl" "demo" {
  # origin a
  /* primary */
  backend {
    name = "a"
  } /* Primary origin */

  /*
    origin b
  */
  backend {
    name = "b"
  }

  /* origin c */
  backend {
    name = "c"
  } /* trailing c */ # and more
}
`
	if string(got) != want {
		t.Fatalf("unexpected order:\n%s", got)
	}
}

func TestSortBlocksByNameRejectsMultilineTrailingComment(t *testing.T) {
	t.Parallel()
	source := []byte("resource \"fastly_service_vcl\" \"demo\" {\n  backend {\n    name = \"b\"\n  } /* spans\n  lines */\n  backend {\n    name = \"a\"\n  }\n}\n")
	_, err := (SortBlocksByName{ResourceType: "fastly_service_vcl", ResourceName: "demo", Block: "backend"}).apply(source, "main.tf")
	if err == nil || !strings.Contains(err.Error(), "additional syntax after its closing brace") {
		t.Fatalf("got %v", err)
	}
}
