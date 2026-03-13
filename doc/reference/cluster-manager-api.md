---
myst:
  html_meta:
    description: REST API reference documentation for the Management API of the MicroCloud Cluster Manager.
---

(ref-cluster-manager-api)=
# Cluster Manager API reference

<link rel="stylesheet" type="text/css" href="../../_static/swagger-ui/swagger-ui.css" ></link>
<link rel="stylesheet" type="text/css" href="../../_static/swagger-override.css" ></link>
<div id="swagger-ui"></div>

<script src="../../_static/swagger-ui/swagger-ui-bundle.js" charset="UTF-8"> </script>
<script src="../../_static/swagger-ui/swagger-ui-standalone-preset.js" charset="UTF-8"> </script>
<script>
window.onload = function() {
  // Begin Swagger UI call region
  const ui = SwaggerUIBundle({
    url: window.location.pathname +"../../cluster-manager-api.yaml",
    dom_id: '#swagger-ui',
    deepLinking: true,
    presets: [
      SwaggerUIBundle.presets.apis,
      SwaggerUIStandalonePreset
    ],
    plugins: [],
    validatorUrl: "none",
    defaultModelsExpandDepth: -1,
    supportedSubmitMethods: []
  })
  // End Swagger UI call region

  window.ui = ui
}
</script>
