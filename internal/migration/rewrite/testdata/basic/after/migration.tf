moved {
  from = fastly_service_vcl.demo
  to   = fastly_service_cdn_auto.demo
}

import {
  to = fastly_service_product_domain_inspector.demo
  id = fastly_service_cdn_auto.demo.id
}
