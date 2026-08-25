resource "fastly_ngwaf_workspace" "test" {
  name        = "{{.WORKSPACE_NAME}}"
  description = "Test NGWAF Workspace for rules"
  mode        = "log"

  attack_signal_thresholds {}
}

resource "fastly_ngwaf_workspace_rule" "test" {
  workspace_id = fastly_ngwaf_workspace.test.id
  type         = "request"
  description  = "Group and multival conditions"
  enabled      = true

  group_condition {
    group_operator = "any"

    condition {
      field    = "ip"
      operator = "equals"
      value    = "127.0.0.1"
    }

    multival_condition {
      field          = "request_header"
      operator       = "exists"
      group_operator = "all"

      condition {
        field    = "name"
        operator = "equals"
        value    = "X-Forwarded-For"
      }
    }
  }

  multival_condition {
    field          = "query_parameter"
    operator       = "exists"
    group_operator = "any"

    condition {
      field    = "name"
      operator = "equals"
      value    = "debug"
    }
  }

  action {
    type = "allow"
  }
}
