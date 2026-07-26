import "@typespec/http";
import "@typespec/openapi";
import "@typespec/openapi3";

import "./models.tsp";
import "./routes.tsp";

using Http;
using OpenAPI;

/**
 * birb — a mobile-first, offline-capable bird-watching API.
 *
 * All endpoints require a Supabase-issued JWT in `Authorization: Bearer <token>`.
 * The Go service verifies the token against the Supabase project JWKS and
 * provisions the user record just-in-time on the first authenticated request.
 */
@service(#{ title: "birb API" })
@info(#{ version: "0.1.0" })
@server("http://localhost:8080", "Local development")
@server("https://api.birb.example", "Production")
@useAuth(BearerAuth)
@route("/api")
namespace Birb;
