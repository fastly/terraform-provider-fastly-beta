package rewrite

import (
	"testing"
)

func TestReplaceLiteral(t *testing.T) {
	t.Parallel()
	source := []byte(`resource "fastly_service_vcl" "demo" {
  director {
    name = "a"
    type = 3 # keep
  }
  director {
    name = "b"
    type = "4"
  }
  director {
    name = "c"
    type = var.type
  }
  director {
    name = "d"
  }
  director {
    name = "e"
    type = 2
  }
  cache_setting {
    name   = "x"
    action = "PASS"
  }
  cache_setting {
    name   = "y"
    action = "pass"
  }
}
`)
	directors := ReplaceLiteral{
		ResourceType: "fastly_service_vcl", ResourceName: "demo",
		Parents: []string{"director"}, Attribute: "type",
		Values: map[string]string{"1": "random", "3": "hash", "4": "client"},
	}
	got, err := directors.apply(source, "main.tf")
	if err != nil {
		t.Fatal(err)
	}
	actions := ReplaceLiteral{
		ResourceType: "fastly_service_vcl", ResourceName: "demo",
		Parents: []string{"cache_setting"}, Attribute: "action",
		Values:          map[string]string{"cache": "cache", "pass": "pass", "restart": "restart"},
		CaseInsensitive: true,
	}
	got, err = actions.apply(got, "main.tf")
	if err != nil {
		t.Fatal(err)
	}
	want := `resource "fastly_service_vcl" "demo" {
  director {
    name = "a"
    type = "hash" # keep
  }
  director {
    name = "b"
    type = "client"
  }
  director {
    name = "c"
    type = var.type
  }
  director {
    name = "d"
  }
  director {
    name = "e"
    type = 2
  }
  cache_setting {
    name   = "x"
    action = "pass"
  }
  cache_setting {
    name   = "y"
    action = "pass"
  }
}
`
	if string(got) != want {
		t.Fatalf("unexpected rewrite:\n%s", got)
	}
}

func TestRemoveLiteralString(t *testing.T) {
	t.Parallel()
	source := []byte(`resource "fastly_service_vcl" "demo" {
  gzip {
    name = "a"
    # attached comment
    cache_condition = ""
  }
  gzip {
    name            = "b"
    cache_condition = "cond"
  }
  gzip {
    name            = "c"
    cache_condition = var.cond
  }
}
`)
	got, err := (RemoveLiteralString{
		ResourceType: "fastly_service_vcl", ResourceName: "demo",
		Parents: []string{"gzip"}, Attribute: "cache_condition",
	}).apply(source, "main.tf")
	if err != nil {
		t.Fatal(err)
	}
	want := `resource "fastly_service_vcl" "demo" {
  gzip {
    name = "a"
  }
  gzip {
    name            = "b"
    cache_condition = "cond"
  }
  gzip {
    name            = "c"
    cache_condition = var.cond
  }
}
`
	if string(got) != want {
		t.Fatalf("unexpected rewrite:\n%s", got)
	}
}
