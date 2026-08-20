resource "fastly_custom_dashboard" "test" {
  name        = "{{.DASHBOARD_NAME}}"
  description = "{{.DASHBOARD_DESCRIPTION}}"

  dashboard_item {
    key      = "item1"
    title    = "Chart #1"
    subtitle = "First chart"
    span     = 4

    data_source {
      type = "stats.edge"
      config {
        metrics = ["requests"]
      }
    }

    visualization {
      config {
        plot_type = "bar"
      }
    }
  }

  dashboard_item {
    key      = "item2"
    title    = "Chart #2"
    subtitle = "Second chart"
    span     = 12

    data_source {
      type = "stats.domain"
      config {
        metrics = ["status_4xx", "status_5xx"]
      }
    }

    visualization {
      config {
        plot_type          = "line"
        calculation_method = "avg"
      }
    }
  }
}
