package docs

const OpenAPIJSON = `{
  "openapi": "3.0.3",
  "info": {
    "title": "Goban API",
    "version": "1.0.0",
    "description": "Authenticated API for Bilibili comment monitoring, keyword rules, reports, settings, and account management."
  },
  "servers": [
    {
      "url": "/"
    }
  ],
  "security": [
    {
      "basicAuth": []
    }
  ],
  "components": {
    "securitySchemes": {
      "basicAuth": {
        "type": "http",
        "scheme": "basic"
      }
    },
    "schemas": {
      "ErrorResponse": {
        "type": "object",
        "properties": {
          "error": {
            "type": "string"
          }
        }
      },
      "MessageResponse": {
        "type": "object",
        "properties": {
          "message": {
            "type": "string"
          }
        }
      },
      "BiliUser": {
        "type": "object",
        "properties": {
          "id": { "type": "integer" },
          "uid": { "type": "integer", "format": "int64" },
          "uname": { "type": "string" },
          "face": { "type": "string" },
          "login": { "type": "boolean" },
          "level": { "type": "integer" },
          "cookie_status": { "type": "string" },
          "cookie_message": { "type": "string" },
          "last_cookie_check": { "type": "string", "format": "date-time", "nullable": true }
        }
      },
      "MonitorTask": {
        "type": "object",
        "properties": {
          "id": { "type": "integer" },
          "user_id": { "type": "integer" },
          "name": { "type": "string" },
          "targets": {
            "type": "array",
            "items": { "$ref": "#/components/schemas/MonitorTarget" }
          },
          "video_count": { "type": "integer" },
          "comment_count": { "type": "integer" },
          "keywords": { "type": "string" },
          "keyword_rule_ids": { "type": "string" },
          "enabled": { "type": "boolean" },
          "interval": { "type": "integer" },
          "report_delay": { "type": "integer" },
          "daily_report_limit": { "type": "integer" },
          "max_retries": { "type": "integer" },
          "retry_interval": { "type": "integer" },
          "proxy_url": { "type": "string" },
          "last_status": { "type": "string" },
          "last_error": { "type": "string" },
          "next_run_at": { "type": "string", "format": "date-time", "nullable": true },
          "backoff_until": { "type": "string", "format": "date-time", "nullable": true },
          "backoff_reason": { "type": "string" },
          "backoff_attempt": { "type": "integer" },
          "progress_total": { "type": "integer", "format": "int64" },
          "progress_done": { "type": "integer", "format": "int64" },
          "progress_message": { "type": "string" },
          "checked_comments": { "type": "integer", "format": "int64" },
          "matched_comments": { "type": "integer", "format": "int64" },
          "report_count": { "type": "integer", "format": "int64" }
        }
      },
      "MonitorTarget": {
        "type": "object",
        "properties": {
          "id": { "type": "integer" },
          "task_id": { "type": "integer" },
          "uid": { "type": "integer", "format": "int64" },
          "uname": { "type": "string" },
          "last_check": { "type": "string", "format": "date-time" },
          "last_status": { "type": "string" },
          "last_error": { "type": "string" },
          "checked_comments": { "type": "integer", "format": "int64" },
          "matched_comments": { "type": "integer", "format": "int64" },
          "report_count": { "type": "integer", "format": "int64" }
        }
      },
      "KeywordRule": {
        "type": "object",
        "properties": {
          "id": { "type": "integer" },
          "name": { "type": "string" },
          "pattern": { "type": "string" },
          "match_type": { "type": "string", "enum": ["plain", "regex"] },
          "match_logic": { "type": "string", "enum": ["single", "or", "and"] },
          "case_sensitive": { "type": "boolean" },
          "enabled": { "type": "boolean" },
          "description": { "type": "string" },
          "last_matched_at": { "type": "string", "format": "date-time", "nullable": true }
        }
      },
      "WhitelistUser": {
        "type": "object",
        "properties": {
          "id": { "type": "integer" },
          "uid": { "type": "integer", "format": "int64" },
          "uname": { "type": "string" },
          "remark": { "type": "string" },
          "enabled": { "type": "boolean" }
        }
      },
      "ReportRecord": {
        "type": "object",
        "properties": {
          "id": { "type": "integer" },
          "task_id": { "type": "integer" },
          "target_uid": { "type": "integer", "format": "int64" },
          "target_uname": { "type": "string" },
          "bvid": { "type": "string" },
          "video_title": { "type": "string" },
          "comment_id": { "type": "integer", "format": "int64" },
          "comment_content": { "type": "string" },
          "comment_user": { "type": "string" },
          "matched_keyword": { "type": "string" },
          "keyword_rule_name": { "type": "string" },
          "success": { "type": "boolean" },
          "message": { "type": "string" }
        }
      },
      "MonitorLog": {
        "type": "object",
        "properties": {
          "id": { "type": "integer" },
          "task_id": { "type": "integer" },
          "message": { "type": "string" },
          "level": { "type": "string" },
          "repeat_count": { "type": "integer" },
          "last_seen_at": { "type": "string", "format": "date-time", "nullable": true }
        }
      },
      "TaskProgressItem": {
        "type": "object",
        "properties": {
          "task": { "$ref": "#/components/schemas/MonitorTask" },
          "recent_logs": { "type": "array", "items": { "$ref": "#/components/schemas/MonitorLog" } },
          "progress_percent": { "type": "integer" }
        }
      }
    },
    "parameters": {
      "ID": {
        "name": "id",
        "in": "path",
        "required": true,
        "schema": { "type": "integer" }
      },
      "Page": {
        "name": "page",
        "in": "query",
        "schema": { "type": "integer", "minimum": 1, "default": 1 }
      },
      "PageSize": {
        "name": "page_size",
        "in": "query",
        "schema": { "type": "integer", "minimum": 1, "maximum": 200, "default": 50 }
      }
    },
    "responses": {
      "BadRequest": {
        "description": "Invalid request",
        "content": {
          "application/json": {
            "schema": { "$ref": "#/components/schemas/ErrorResponse" }
          }
        }
      },
      "Unauthorized": {
        "description": "Basic authentication required"
      },
      "RateLimited": {
        "description": "Too many authentication failures; retry later",
        "headers": {
          "Retry-After": {
            "description": "Seconds before retrying authentication",
            "schema": { "type": "integer" }
          }
        },
        "content": {
          "application/json": {
            "schema": { "$ref": "#/components/schemas/ErrorResponse" }
          }
        }
      }
    }
  },
  "paths": {
    "/api/docs": {
      "get": {
        "summary": "Open API documentation UI",
        "tags": ["Docs"],
        "responses": {
          "200": {
            "description": "HTML documentation page",
            "content": { "text/html": { "schema": { "type": "string" } } }
          }
        }
      }
    },
    "/api/docs/openapi.json": {
      "get": {
        "summary": "OpenAPI specification",
        "tags": ["Docs"],
        "responses": {
          "200": {
            "description": "OpenAPI 3 JSON document",
            "content": { "application/json": { "schema": { "type": "object" } } }
          }
        }
      }
    },
    "/api/users/list": {
      "get": {
        "summary": "List Bilibili accounts",
        "tags": ["Users"],
        "responses": {
          "200": {
            "description": "Account list",
            "content": { "application/json": { "schema": { "type": "array", "items": { "$ref": "#/components/schemas/BiliUser" } } } }
          },
          "429": { "$ref": "#/components/responses/RateLimited" }
        }
      }
    },
    "/api/users/login": {
      "get": {
        "summary": "Create Bilibili QR login session",
        "tags": ["Users"],
        "responses": { "200": { "description": "QR image and session key" } }
      }
    },
    "/api/users/loginCheck": {
      "get": {
        "summary": "Poll QR login status",
        "tags": ["Users"],
        "parameters": [
          { "name": "key", "in": "query", "required": true, "schema": { "type": "string" } }
        ],
        "responses": { "200": { "description": "Login status" } }
      }
    },
    "/api/users/loginCancel": {
      "get": {
        "summary": "Cancel QR login session",
        "tags": ["Users"],
        "parameters": [
          { "name": "key", "in": "query", "schema": { "type": "string" } }
        ],
        "responses": { "200": { "description": "Session cancelled" } }
      }
    },
    "/api/users/loginByCookie": {
      "post": {
        "summary": "Login Bilibili account by Cookie",
        "tags": ["Users"],
        "requestBody": {
          "required": true,
          "content": { "application/json": { "schema": { "type": "object", "required": ["cookies"], "properties": { "cookies": { "type": "string" } } } } }
        },
        "responses": { "200": { "description": "Login result" } }
      }
    },
    "/api/users/{id}/check": {
      "post": {
        "summary": "Check account Cookie validity",
        "tags": ["Users"],
        "parameters": [{ "$ref": "#/components/parameters/ID" }],
        "responses": { "200": { "description": "Cookie check result" } }
      }
    },
    "/api/users/{id}": {
      "delete": {
        "summary": "Delete Bilibili account",
        "tags": ["Users"],
        "parameters": [{ "$ref": "#/components/parameters/ID" }],
        "responses": { "200": { "description": "Delete result" } }
      }
    },
    "/api/tasks/list": {
      "get": {
        "summary": "List monitor tasks",
        "tags": ["Tasks"],
        "responses": { "200": { "description": "Task list", "content": { "application/json": { "schema": { "type": "array", "items": { "$ref": "#/components/schemas/MonitorTask" } } } } } }
      }
    },
    "/api/tasks/progress": {
      "get": {
        "summary": "List monitor task progress with recent logs",
        "tags": ["Tasks"],
        "responses": { "200": { "description": "Task progress list", "content": { "application/json": { "schema": { "type": "array", "items": { "$ref": "#/components/schemas/TaskProgressItem" } } } } } }
      }
    },
    "/api/tasks/create": {
      "post": {
        "summary": "Create monitor task",
        "tags": ["Tasks"],
        "responses": { "200": { "description": "Created task" }, "400": { "$ref": "#/components/responses/BadRequest" } }
      }
    },
    "/api/tasks/{id}": {
      "put": {
        "summary": "Update monitor task",
        "tags": ["Tasks"],
        "parameters": [{ "$ref": "#/components/parameters/ID" }],
        "responses": { "200": { "description": "Updated task" } }
      },
      "delete": {
        "summary": "Delete monitor task",
        "tags": ["Tasks"],
        "parameters": [{ "$ref": "#/components/parameters/ID" }],
        "responses": { "200": { "description": "Delete result" } }
      }
    },
    "/api/tasks/{id}/progress": {
      "get": {
        "summary": "Get one monitor task progress with recent logs",
        "tags": ["Tasks"],
        "parameters": [{ "$ref": "#/components/parameters/ID" }],
        "responses": { "200": { "description": "Task progress", "content": { "application/json": { "schema": { "$ref": "#/components/schemas/TaskProgressItem" } } } } }
      }
    },
    "/api/tasks/{id}/status": {
      "post": {
        "summary": "Update task status or scheduling state",
        "tags": ["Tasks"],
        "parameters": [{ "$ref": "#/components/parameters/ID" }],
        "responses": { "200": { "description": "Updated task state" }, "400": { "$ref": "#/components/responses/BadRequest" } }
      }
    },
    "/api/tasks/{id}/test": {
      "get": {
        "summary": "Run a one-shot monitor task test",
        "tags": ["Tasks"],
        "parameters": [{ "$ref": "#/components/parameters/ID" }],
        "responses": { "200": { "description": "Preview matches and compile errors" } }
      }
    },
    "/api/keywords/list": {
      "get": {
        "summary": "List keyword rules",
        "tags": ["Keywords"],
        "responses": { "200": { "description": "Keyword rule list", "content": { "application/json": { "schema": { "type": "array", "items": { "$ref": "#/components/schemas/KeywordRule" } } } } } }
      }
    },
    "/api/keywords/create": {
      "post": {
        "summary": "Create keyword rule",
        "tags": ["Keywords"],
        "responses": { "200": { "description": "Created rule" }, "400": { "$ref": "#/components/responses/BadRequest" } }
      }
    },
    "/api/keywords/preview": {
      "post": {
        "summary": "Preview keyword matching",
        "tags": ["Keywords"],
        "responses": { "200": { "description": "Match preview" } }
      }
    },
    "/api/keywords/{id}": {
      "put": {
        "summary": "Update keyword rule",
        "tags": ["Keywords"],
        "parameters": [{ "$ref": "#/components/parameters/ID" }],
        "responses": { "200": { "description": "Updated rule" } }
      },
      "delete": {
        "summary": "Delete keyword rule",
        "tags": ["Keywords"],
        "parameters": [{ "$ref": "#/components/parameters/ID" }],
        "responses": { "200": { "description": "Delete result" } }
      }
    },
    "/api/whitelist/list": {
      "get": {
        "summary": "List whitelist users",
        "tags": ["Whitelist"],
        "responses": { "200": { "description": "Whitelist", "content": { "application/json": { "schema": { "type": "array", "items": { "$ref": "#/components/schemas/WhitelistUser" } } } } } }
      }
    },
    "/api/whitelist/create": {
      "post": {
        "summary": "Create whitelist user",
        "tags": ["Whitelist"],
        "responses": { "200": { "description": "Created whitelist entry" } }
      }
    },
    "/api/whitelist/{id}": {
      "put": {
        "summary": "Update whitelist user",
        "tags": ["Whitelist"],
        "parameters": [{ "$ref": "#/components/parameters/ID" }],
        "responses": { "200": { "description": "Updated whitelist entry" } }
      },
      "delete": {
        "summary": "Delete whitelist user",
        "tags": ["Whitelist"],
        "parameters": [{ "$ref": "#/components/parameters/ID" }],
        "responses": { "200": { "description": "Delete result" } }
      }
    },
    "/api/settings": {
      "get": {
        "summary": "Get runtime and persisted settings",
        "tags": ["Settings"],
        "responses": { "200": { "description": "Settings" } }
      },
      "put": {
        "summary": "Update settings",
        "tags": ["Settings"],
        "responses": { "200": { "description": "Updated settings" }, "400": { "$ref": "#/components/responses/BadRequest" } }
      }
    },
    "/api/status": {
      "get": {
        "summary": "Get monitor status summary",
        "tags": ["Status"],
        "responses": { "200": { "description": "Status summary" } }
      }
    },
    "/api/logs/monitor": {
      "get": {
        "summary": "List monitor logs",
        "tags": ["Logs"],
        "parameters": [{ "$ref": "#/components/parameters/Page" }, { "$ref": "#/components/parameters/PageSize" }],
        "responses": { "200": { "description": "Paginated monitor logs" } }
      }
    },
    "/api/logs/report": {
      "get": {
        "summary": "List report records",
        "tags": ["Reports"],
        "parameters": [{ "$ref": "#/components/parameters/Page" }, { "$ref": "#/components/parameters/PageSize" }],
        "responses": { "200": { "description": "Paginated report records" } }
      }
    },
    "/api/logs/report/export": {
      "get": {
        "summary": "Export report records as CSV",
        "tags": ["Reports"],
        "responses": { "200": { "description": "CSV export", "content": { "text/csv": { "schema": { "type": "string" } } } } }
      }
    },
    "/health": {
      "get": {
        "summary": "Health check",
        "security": [],
        "tags": ["Health"],
        "responses": { "200": { "description": "Service is healthy" } }
      }
    }
  }
}`

const SwaggerHTML = `<!doctype html>
<html lang="zh-CN">
<head>
  <meta charset="utf-8">
  <meta name="viewport" content="width=device-width, initial-scale=1">
  <title>Goban API Docs</title>
  <style>
    body { margin: 0; font-family: -apple-system, BlinkMacSystemFont, "Segoe UI", sans-serif; background: #f6f8fb; color: #1f2933; }
    header { background: #1f2937; color: white; padding: 18px 24px; }
    main { max-width: 1080px; margin: 0 auto; padding: 24px; }
    section { background: white; border: 1px solid #d9e2ec; border-radius: 8px; margin-bottom: 14px; overflow: hidden; }
    h1 { margin: 0; font-size: 22px; }
    h2 { margin: 0; padding: 14px 16px; font-size: 16px; background: #eef2f7; }
    .route { display: grid; grid-template-columns: 86px 1fr; gap: 10px; padding: 12px 16px; border-top: 1px solid #edf2f7; align-items: center; }
    .method { font-weight: 700; color: white; border-radius: 4px; padding: 5px 8px; text-align: center; font-size: 12px; }
    .GET { background: #2563eb; }
    .POST { background: #059669; }
    .PUT { background: #d97706; }
    .DELETE { background: #dc2626; }
    code { font-family: ui-monospace, SFMono-Regular, Menlo, monospace; font-size: 13px; }
    a { color: #2563eb; }
  </style>
</head>
<body>
  <header>
    <h1>Goban API Docs</h1>
  </header>
  <main>
    <p>OpenAPI JSON: <a href="/api/docs/openapi.json">/api/docs/openapi.json</a>. This page is served under the authenticated API group.</p>
    <div id="content">Loading...</div>
  </main>
  <script>
    fetch('/api/docs/openapi.json')
      .then(function (res) { return res.json(); })
      .then(function (spec) {
        var grouped = {};
        Object.keys(spec.paths).forEach(function (path) {
          Object.keys(spec.paths[path]).forEach(function (method) {
            var op = spec.paths[path][method];
            var tag = (op.tags && op.tags[0]) || 'Other';
            grouped[tag] = grouped[tag] || [];
            grouped[tag].push({ method: method.toUpperCase(), path: path, summary: op.summary || '' });
          });
        });
        document.getElementById('content').innerHTML = Object.keys(grouped).sort().map(function (tag) {
          return '<section><h2>' + tag + '</h2>' + grouped[tag].map(function (route) {
            return '<div class="route"><span class="method ' + route.method + '">' + route.method + '</span><div><code>' + route.path + '</code><br>' + route.summary + '</div></div>';
          }).join('') + '</section>';
        }).join('');
      })
      .catch(function (err) {
        document.getElementById('content').textContent = 'Failed to load OpenAPI spec: ' + err.message;
      });
  </script>
</body>
</html>`
