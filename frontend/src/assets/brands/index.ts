type BrandShape =
  | { tag: "path"; attrs: { d: string; "fill-rule"?: "evenodd" | "nonzero"; "clip-rule"?: "evenodd" | "nonzero" } }
  | { tag: "rect"; attrs: { x: number; y: number; width: number; height: number; rx: number } };

export interface BrandDefinition {
  name: string;
  viewBox: string;
  color: string;
  source: string;
  license: string;
  style: "outline" | "solid";
  defaultStrokeWidth?: number;
  shapes: BrandShape[];
}

export const brands = {
  bilibili: {
    name: "Bilibili", viewBox: "0 0 48 48", color: "#00a1d6",
    source: "https://icon-sets.iconify.design/arcticons/bilibili/",
    license: "CC-BY-SA-4.0", style: "outline", defaultStrokeWidth: 2.5,
    shapes: [
      { tag: "rect", attrs: { width: 35.733, height: 26.558, x: 6.134, y: 9.403, rx: 6.753 } },
      { tag: "path", attrs: { d: "m17.483 4.5l2.925 4.903m-7.149 12.282l6.824-2.726M30.517 4.5l-2.925 4.903m7.149 12.282l-6.824-2.726m1.137 5.957c-.582 1.472-1.15 2.54-2.527 2.54c-1.074 0-1.666-.498-2.527-1.836c-.861 1.338-1.453 1.836-2.527 1.836c-1.377 0-1.945-1.068-2.527-2.54m7.022 11.045v2.033s8.691.29 8.691 5.506H13.341c0-5.217 8.691-5.506 8.691-5.506v-2.033" } },
    ],
  },
  lofter: {
    name: "LOFTER", viewBox: "0 0 48 48", color: "#2dbe60",
    source: "https://icon-sets.iconify.design/arcticons/lofter/",
    license: "CC-BY-SA-4.0", style: "outline", defaultStrokeWidth: 3,
    shapes: [
      { tag: "path", attrs: { d: "m21.074 29.336l-.162-20.194c.201-3.091-6.673-7.562-8.24-4.2l.162 22.294c-.104 6.994 4.915 6.81 8.24 7.27h10.824c5.004-.1 7.402 7.39 4.523 8.077l-8.4.162c-3.18-.093-5.614-1.387-6.463-3.877" } },
    ],
  },
  youtube: {
    name: "YouTube", viewBox: "0 0 48 48", color: "#ff0000",
    source: "https://icon-sets.iconify.design/arcticons/youtube/",
    license: "CC-BY-SA-4.0", style: "outline", defaultStrokeWidth: 2.5,
    shapes: [
      { tag: "path", attrs: { d: "M43.2 14.5a5.02 5.02 0 0 0-3.53-3.55C36.56 10.1 24 10.1 24 10.1s-12.56 0-15.67.85A5.02 5.02 0 0 0 4.8 14.5C4 17.64 4 24 4 24s0 6.36.8 9.5a5.02 5.02 0 0 0 3.53 3.55c3.11.85 15.67.85 15.67.85s12.56 0 15.67-.85a5.02 5.02 0 0 0 3.53-3.55C44 30.36 44 24 44 24s0-6.36-.8-9.5Z" } },
      { tag: "path", attrs: { d: "M20 29.8V18.2L30.4 24L20 29.8Z" } },
    ],
  },
} satisfies Record<string, BrandDefinition>;

export type BrandName = keyof typeof brands;
