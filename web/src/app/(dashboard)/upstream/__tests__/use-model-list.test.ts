import { describe, expect, it } from "vitest";
import { buildModelListParams, nextSortState } from "../model-list-params";

describe("buildModelListParams", () => {
  it("omits empty optional params", () => {
    const p = buildModelListParams({
      page: 2,
      pageSize: 20,
      freeText: "",
      params: {},
      sortField: "created_at",
      sort: "desc",
    });
    expect(p).toEqual({ page: 2, pageSize: 20, sortField: "created_at", sort: "desc" });
  });

  it("maps facet params to api params", () => {
    const p = buildModelListParams({
      page: 1,
      pageSize: 10,
      freeText: "gpt",
      params: { status: "disabled", capability: "image", username: "alice" },
      sortField: "alias",
      sort: "asc",
    });
    expect(p.query).toBe("gpt");
    expect(p.status).toBe("disabled");
    expect(p.capability).toBe("image");
    expect(p.username).toBe("alice");
  });

  it("drops status/capability values outside the backend whitelist", () => {
    const p = buildModelListParams({
      page: 1,
      pageSize: 10,
      freeText: "",
      params: { status: "bogus", capability: "audio" },
      sortField: "alias",
      sort: "asc",
    });
    expect(p.status).toBeUndefined();
    expect(p.capability).toBeUndefined();
  });

  it("omits endpointID when zero or unparsable", () => {
    expect(
      buildModelListParams({
        page: 1,
        pageSize: 10,
        freeText: "",
        params: { endpoint: "0" },
        sortField: "alias",
        sort: "asc",
      }).endpointID,
    ).toBeUndefined();
    expect(
      buildModelListParams({
        page: 1,
        pageSize: 10,
        freeText: "",
        params: { endpoint: "abc" },
        sortField: "alias",
        sort: "asc",
      }).endpointID,
    ).toBeUndefined();
    expect(
      buildModelListParams({
        page: 1,
        pageSize: 10,
        freeText: "",
        params: { endpoint: "7" },
        sortField: "alias",
        sort: "asc",
      }).endpointID,
    ).toBe(7);
  });
});

describe("nextSortState", () => {
  it("toggles direction when clicking the active column", () => {
    expect(nextSortState({ sortField: "alias", sort: "asc" }, "alias")).toEqual({
      sortField: "alias",
      sort: "desc",
    });
  });

  it("switches column and resets to asc", () => {
    expect(nextSortState({ sortField: "alias", sort: "desc" }, "created_at")).toEqual({
      sortField: "created_at",
      sort: "asc",
    });
  });
});
