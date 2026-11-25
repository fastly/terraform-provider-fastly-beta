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

func TestNormalizeStringExpression(t *testing.T) {
	t.Parallel()
	source := []byte(`resource "fastly_service_vcl" "demo" {
  cache_setting {
    name            = "literal"
    action          = "PASS"
    cache_condition = "c"
  }
  cache_setting {
    name            = "variable"
    action          = var.action # keep
    cache_condition = var.condition
  }
  cache_setting {
    name            = "conditional"
    action          = var.pass ? "PASS" : null
    cache_condition = var.on ? "c" : ""
  }
  cache_setting {
    name            = "lowered"
    action          = lower(var.action)
    cache_condition = lower(var.condition)
  }
  cache_setting {
    name            = "null"
    action          = null
    cache_condition = null
  }
  cache_setting {
    name = "omitted"
  }
}
`)
	got, err := (NormalizeStringExpression{
		ResourceType: "fastly_service_vcl", ResourceName: "demo",
		Parents: []string{"cache_setting"}, Attribute: "action", Lowercase: true,
	}).apply(source, "main.tf")
	if err != nil {
		t.Fatal(err)
	}
	got, err = (NormalizeStringExpression{
		ResourceType: "fastly_service_vcl", ResourceName: "demo",
		Parents: []string{"cache_setting"}, Attribute: "cache_condition",
	}).apply(got, "main.tf")
	if err != nil {
		t.Fatal(err)
	}
	want := `resource "fastly_service_vcl" "demo" {
  cache_setting {
    name            = "literal"
    action          = "PASS"
    cache_condition = "c"
  }
  cache_setting {
    name            = "variable"
    action          = var.action == null || var.action == "" ? null : lower(var.action) # keep
    cache_condition = var.condition == "" ? null : var.condition
  }
  cache_setting {
    name            = "conditional"
    action          = (var.pass ? "PASS" : null) == null || (var.pass ? "PASS" : null) == "" ? null : lower(var.pass ? "PASS" : null)
    cache_condition = (var.on ? "c" : "") == "" ? null : (var.on ? "c" : "")
  }
  cache_setting {
    name            = "lowered"
    action          = lower(var.action) == "" ? null : lower(var.action)
    cache_condition = lower(var.condition) == "" ? null : lower(var.condition)
  }
  cache_setting {
    name            = "null"
    action          = null
    cache_condition = null
  }
  cache_setting {
    name = "omitted"
  }
}
`
	if string(got) != want {
		t.Fatalf("unexpected rewrite:\n%s", got)
	}
}

func TestNormalizeEmptyList(t *testing.T) {
	t.Parallel()
	source := []byte(`resource "fastly_service_vcl" "demo" {
  gzip {
    name = "empty"
    # Explicitly empty.
    extensions = []
  }
  gzip {
    name       = "set"
    extensions = ["css"]
  }
  gzip {
    name       = "variable"
    extensions = var.extensions # keep
  }
  gzip {
    name       = "conditional"
    extensions = var.on ? ["css"] : []
  }
  gzip {
    name       = "null"
    extensions = null
  }
  gzip {
    name       = "for_empty"
    extensions = [for item in [] : item]
  }
  gzip {
    name       = "parenthesized"
    extensions = ([])
  }
  gzip {
    name       = "constant_conditional"
    extensions = true ? [] : ["css"]
  }
  gzip {
    name       = "for_set"
    extensions = [for item in ["css"] : item]
  }
  gzip {
    name = "omitted"
  }
}
`)
	edit := NormalizeEmptyList{
		ResourceType: "fastly_service_vcl", ResourceName: "demo",
		Parents: []string{"gzip"}, Attribute: "extensions",
	}
	got, err := edit.apply(source, "main.tf")
	if err != nil {
		t.Fatal(err)
	}
	want := `resource "fastly_service_vcl" "demo" {
  gzip {
    name = "empty"
  }
  gzip {
    name       = "set"
    extensions = ["css"]
  }
  gzip {
    name       = "variable"
    extensions = var.extensions == null ? null : (length(var.extensions) == 0 ? null : var.extensions) # keep
  }
  gzip {
    name       = "conditional"
    extensions = (var.on ? ["css"] : []) == null ? null : (length(var.on ? ["css"] : []) == 0 ? null : (var.on ? ["css"] : []))
  }
  gzip {
    name       = "null"
    extensions = null
  }
  gzip {
    name       = "for_empty"
  }
  gzip {
    name       = "parenthesized"
  }
  gzip {
    name       = "constant_conditional"
  }
  gzip {
    name       = "for_set"
    extensions = [for item in ["css"] : item]
  }
  gzip {
    name = "omitted"
  }
}
`
	if string(got) != want {
		t.Fatalf("unexpected rewrite:\n%s", got)
	}
}
