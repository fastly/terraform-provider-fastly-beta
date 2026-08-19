resource "fastly_custom_dashboard" "test" {
  name        = "{{.DASHBOARD_NAME}}"
  description = "{{.DASHBOARD_DESCRIPTION}}"

  # Existing item2 moves from index 1 to index 0. Its Fastly ID must remain
  # attached to key "item2", not to whichever block occupies the old index.
  dashboard_item {
    key      = "item2"
    title    = "Chart #2"
    subtitle = "Second chart updated"
    span     = 6

    data_source {
      type = "stats.domain"
      config {
        metrics = ["status_4xx", "status_5xx"]
      }
    }

    visualization {
      type = "chart"
      config {
        plot_type = "donut"
      }
    }
  }

  # New key => new Fastly dashboard item; no API ID is configured.
  dashboard_item {
    key      = "item3"
    title    = "Chart #3"
    subtitle = "New chart"
    span     = 4

    data_source {
      type = "stats.origin"
      config {
        metrics = ["all_status_2xx"]
      }
    }

    visualization {
      type = "chart"
      config {
        plot_type = "single-metric"
      }
    }
  }
}
