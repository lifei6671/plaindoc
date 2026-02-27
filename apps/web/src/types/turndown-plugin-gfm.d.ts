declare module "turndown-plugin-gfm" {
  export const gfm: (service: unknown) => void;
  export const highlightedCodeBlock: (service: unknown) => void;
  export const strikethrough: (service: unknown) => void;
  export const tables: (service: unknown) => void;
  export const taskListItems: (service: unknown) => void;
}
