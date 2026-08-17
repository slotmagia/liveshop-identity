/// <reference types="vite/client" />

// Vite serves `?inline` imports as the stylesheet text instead of emitting a
// separate file. Not every Vite version ships this declaration, so the module
// carries its own rather than depending on which version is installed.
declare module '*.css?inline' {
  const css: string
  export default css
}
