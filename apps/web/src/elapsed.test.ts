import { describe, expect, it } from "vitest";
import { formatElapsed } from "./elapsed";

describe("formatElapsed", () => {
  it("formats seconds as m:ss", () => {
    expect(formatElapsed(0)).toBe("0:00");
    expect(formatElapsed(1000)).toBe("0:01");
    expect(formatElapsed(59_000)).toBe("0:59");
    expect(formatElapsed(60_000)).toBe("1:00");
    expect(formatElapsed(61_500)).toBe("1:01");
    expect(formatElapsed(754_000)).toBe("12:34");
  });

  it("formats hours as h:mm:ss", () => {
    expect(formatElapsed(3_600_000)).toBe("1:00:00");
    expect(formatElapsed(3_661_000)).toBe("1:01:01");
  });

  it("clamps negative durations to zero", () => {
    expect(formatElapsed(-5000)).toBe("0:00");
  });
});
