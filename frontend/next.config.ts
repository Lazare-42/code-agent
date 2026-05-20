import type { NextConfig } from "next"

const BACKEND_URL = process.env.BACKEND_URL || "http://127.0.0.1:8091"

const nextConfig: NextConfig = {
  output: "standalone",
  async rewrites() {
    return [
      {
        source: "/api/:path*",
        destination: `${BACKEND_URL}/api/:path*`,
      },
    ]
  },
}

export default nextConfig
