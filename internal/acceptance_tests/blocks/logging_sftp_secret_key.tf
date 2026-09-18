resource "fastly_service_logging_sftp" "test" {
  service_id      = fastly_service_cdn.test.id
  version         = {{.SERVICE_VERSION}}
  name            = "{{.LOGGING_SFTP_NAME}}"
  address         = "sftp.example.com"
  path            = "/"
  ssh_known_hosts = "sftp.example.com"
  authentication = {
    user = "test-user"
    secret_key = trimspace(<<-EOT
      -----BEGIN PRIVATE KEY-----
      MIIEvgIBADANBgkqhkiG9w0BAQEFAASCBKgwggSkAgEAAoIBAQC1XS1YV2sup97f
      os2kEi5zqsgqIvnTzibCOPE9haS0A0bwJfQDv8DAbVtQtrpVgnGZmg/Z8oYgteFQ
      tY8bWS7QYsF/o/gKJNTDL+4gWsTZFzjg6LH3Z6TW9vpxqsN0zwpQS3sqxwc4YOcF
      FbgOgBzt+LOgmaplyCD7a+wtiBItYlify8eTlGjXoOs0Z2Gq7iJ/uNETvzV4sEoj
      8qPDt6zSQhJ6WCTzAc2ypJNHrtyLizMwV4MVFvAHRJ/BjVJIibLwaanZWrfc2CK+
      4yqA+/UPZeUi5Kp0Q3UdGo0E3YLfzyimy0W+ojYR5imY6kJx9vrWYHH/jImhrv5N
      gWSnyACdAgMBAAECggEAAQf4exCQdWdFJV8kH6slORuhSTV46aaMOTj3cUz+0uh4
      SlDjNeHRUh/mL8KKxXPCTRXfePv/wibsScXKfGS2YFFttNnS6Zb+qRvcwrGwRuT7
      zFay63k2fKzScaB/hqF0AGE5e5SZ499GmPkNrLu3YK0hxb5foWO6E2veDWylqYYr
      mjXzpPrcu+MHbisJO595813Qp0RqzGfyVQ5qt9U69LEhxtG0/7yKQIrs67HpkyqK
      FuheANnw9hwfZ/dFOKn0MuoCqAlguf93rXwxIhCt6AAD5R3A3OugIHqSFvHIyuah
      WVsl/Lc+6ZUOqzDTluCs3i8Ui4JVPMCBrs/l9Tp6MQKBgQD8DAqaET1lWOiklhS1
      B4gE05a2ph8ApahLgZSjx2cu4tbmMRCfO3jzf1rNhqdXFlNWEC3PfAkGmzo8RIOB
      WIBFPeTj1peldN8vcJSc1YtbtU6WIz2HqaSt7xoJ9fjp7uGtIuqIonKo8KGKWSyY
      1aGveBRvdiIa0ddrMbQx7eY28QKBgQC4NViZpNXGWR6TYjRda4fHp1Ztmlu53ZXz
      yWHov6sn74Fs86FOsYuPLm4v0hMwA2iEjpZEg3mQXWIM2/Nullq9874q2i2OQj9m
      Uj+zWr38SWLI1dZPss1d6mmh/VZJ5rMPBQVI3vnyZE0DY1LUfBSwrrFjkgOfgnR2
      mQ8XmURcbQKBgQDyOoBV4QuoQvISe0obUMmgGdlWYACblplPN5GqdRDtNoRhZfYb
      kgSDv3l83FQmlgYxSAs+xG3IM5acJRxdSri70ugPL0U+djuoVAH/WBs+X9jO4b9Y
      iekCYDAeMo6uBC5PPqc3+SdIxTn6xAjgOS/SewzoshfEvrbRBkuvUHtXgQKBgFfh
      i21xiFNifQXPWjAfdt23ZbJQa+ZWYo21y7IgjuU0jEiQSqqiZXRfsE28KU9EsP5c
      kDALkVlgU8DSxmZB8PSibl0/TXCLBngoUR+d8PmFgU1TRzUqlnNxvAd+N0Z2e4J0
      4LqNNi1/0IYHQqMAt1Y7YYGhTX0x3aIfD2Ywxr5ZAoGBAKv1INYATAr6TSW6/BWV
      sFgpsSiVWHN8N6CVAgLlbyeTXrfsXINzJ6P4uJqO85UT2V5KeKL23LFt2mK0RdvD
      vyHny9spd6jloEH9YvlSVX8tB6+n4LgHDa07NZpcuzl/5YspWTyyPBGGtfrAE7pc
      CCVlnjhb4d8Xjas5QUTMG0QT
      -----END PRIVATE KEY-----
      EOT
    )
  }
}
