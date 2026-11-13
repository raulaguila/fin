import { describe, expect, it } from "vitest";
import { splitTags } from "./transaction-forms";

describe("splitTags", () => {
  it("lowers, dashes and drops repeats and blanks", () => {
    expect(splitTags(" Viagem SP, casa,, viagem-sp ,Casa")).toEqual(["viagem-sp", "casa"]);
    expect(splitTags("")).toEqual([]);
  });
});
