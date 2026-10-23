package api

import (
	_ "embed"
)

//go:embed openapi.yaml
var OpenAPISpec []byte

// SwaggerUIHTML renders the interactive Swagger UI documentation page.
const SwaggerUIHTML = `<!DOCTYPE html>
<html lang="en">
<head>
  <meta charset="utf-8" />
  <meta name="viewport" content="width=device-width, initial-scale=1" />
  <meta name="description" content="QuickCart Engine API Documentation" />
  <title>QuickCart API - Interactive Documentation</title>
  <link rel="stylesheet" href="https://unpkg.com/swagger-ui-dist@5.11.0/swagger-ui.css" />
  <link rel="icon" type="image/png" href="https://unpkg.com/swagger-ui-dist@5.11.0/favicon-32x32.png" sizes="32x32" />
  <style>
    body {
      margin: 0;
      padding: 0;
      background: #0f172a;
      font-family: -apple-system, BlinkMacSystemFont, "Segoe UI", Roboto, Helvetica, Arial, sans-serif;
    }
    .topbar {
      display: none !important;
    }
    .swagger-ui .info .title {
      color: #38bdf8;
      font-size: 2.2rem;
    }
    .swagger-ui {
      max-width: 1200px;
      margin: 0 auto;
      padding: 24px;
      background: #ffffff;
      border-radius: 8px;
      box-shadow: 0 10px 25px -5px rgba(0, 0, 0, 0.3);
      margin-top: 24px;
      margin-bottom: 48px;
    }
  </style>
</head>
<body>
  <div id="swagger-ui"></div>
  <script src="https://unpkg.com/swagger-ui-dist@5.11.0/swagger-ui-bundle.js" charset="UTF-8"></script>
  <script src="https://unpkg.com/swagger-ui-dist@5.11.0/swagger-ui-standalone-preset.js" charset="UTF-8"></script>
  <script>
    window.onload = () => {
      window.ui = SwaggerUIBundle({
        url: '/api/openapi.yaml',
        dom_id: '#swagger-ui',
        deepLinking: true,
        presets: [
          SwaggerUIBundle.presets.apis,
          SwaggerUIStandalonePreset
        ],
        layout: "BaseLayout"
      });
    };
  </script>
</body>
</html>
`
