/// <reference types="vite/client" />

interface ImportMetaEnv {
  /** Base URL of the Go API. Empty means "same origin as the page". */
  readonly VITE_API_URL?: string;
}

interface ImportMeta {
  readonly env: ImportMetaEnv;
}
