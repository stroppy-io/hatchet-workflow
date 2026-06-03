import { useMemo } from "react";
import { toSvg, type JdenticonConfig } from "jdenticon";
import { cn } from "@/lib/utils";

// Deterministic, name-based avatar in the GitHub identicon style. The same
// `name` always yields the same icon. jdenticon generates the symmetric,
// geometric pixel-art SVG that GitHub popularised — rendered as a static SVG
// string (no DOM observer / canvas), so it stays crisp at any size.
//
// Hues are constrained to the app's accent range (blues / indigo / cyan with a
// touch of green) so identicons sit naturally in the dark theme. The icon is
// drawn on the theme's muted surface rather than transparent so it reads as a
// solid avatar chip at small sizes.
const JDENTICON_CONFIG: JdenticonConfig = {
  hues: [210, 220, 245, 195, 140],
  lightness: {
    color: [0.45, 0.7],
    grayscale: [0.4, 0.6],
  },
  saturation: {
    color: 0.55,
    grayscale: 0.3,
  },
  backColor: "#1a1a2e",
  padding: 0.08,
};

interface AvatarProps {
  name: string;
  /** Pixel size of the square avatar. Defaults to 28. */
  size?: number;
  className?: string;
}

export function Avatar({ name, size = 28, className }: AvatarProps) {
  const svg = useMemo(
    () => toSvg(name, size, JDENTICON_CONFIG),
    [name, size],
  );

  return (
    <span
      className={cn("inline-flex overflow-hidden rounded-full", className)}
      style={{ width: size, height: size }}
      aria-hidden="true"
      dangerouslySetInnerHTML={{ __html: svg }}
    />
  );
}
