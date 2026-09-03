resource "fastly_ngwaf_signal" "signal_1" {
  applies_to  = ["*"]
  name        = "{{.SIGNAL_NAME_1}}"
  description = "First account signal"
}

resource "fastly_ngwaf_signal" "signal_2" {
  applies_to  = ["*"]
  name        = "{{.SIGNAL_NAME_2}}"
  description = "Second account signal"
}

data "fastly_ngwaf_signals" "test" {
  depends_on = [
    fastly_ngwaf_signal.signal_1,
    fastly_ngwaf_signal.signal_2,
  ]
}
