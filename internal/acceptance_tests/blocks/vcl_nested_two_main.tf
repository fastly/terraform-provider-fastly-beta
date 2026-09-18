custom_vcl {
  name    = "main_one"
  main    = true
  content = file("{{.VCL_MAIN_FILE_PATH}}")
}

custom_vcl {
  name    = "main_two"
  main    = true
  content = file("{{.VCL_SECOND_MAIN_FILE_PATH}}")
}
