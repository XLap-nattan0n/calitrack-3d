/** Static config for the 3D anatomy viewer (NAT-73). Source: docs/anatomy-model.md. */
export const ANATOMY_MODEL_URL = "/models/anatomy-muscles.glb";
export const DRACO_DECODER_PATH = "/draco/";

export type Highlight = "primary" | "secondary" | "selected";

export const HIGHLIGHT_COLORS: Record<Highlight | "idle", string> = {
  primary: "#dc2626", // red
  secondary: "#f97316", // orange
  selected: "#2563eb", // blue
  idle: "#d9c2bd",
};

/** Model centre (m) and framing distance; model is ~1.5 m tall, +Y up, front faces +Z. */
export const MODEL_CENTER: [number, number, number] = [0, 0.77, 0.1];
export const CAMERA_DISTANCE = 2.9;
export type ViewPreset = "front" | "back";
