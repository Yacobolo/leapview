// web/components/filter-url.ts
function defaultControl(definition) {
  switch (definition.type) {
    case "date_range":
      return { type: "date_range", preset: definition.default?.preset || "all", from: definition.default?.from || "", to: definition.default?.to || "" };
    case "multi_select":
      return { type: "multi_select", operator: definition.operator || "in", values: [...definition.default?.values ?? []] };
    case "text":
      return { type: "text", operator: definition.default?.operator || definition.defaultOperator || "contains", value: definition.default?.value || "" };
    default:
      return { type: definition.type || "" };
  }
}
function filtersFromURLParams(config, filters, params) {
  const next = {
    controls: { ...filters.controls ?? {} },
    visualSelections: [...filters.visualSelections ?? []]
  };
  for (const [name, definition] of Object.entries(config)) {
    const base = defaultControl(definition);
    const current = next.controls[name] ?? base;
    switch (definition.type) {
      case "date_range":
        next.controls[name] = dateControlFromParams(definition, current, params);
        break;
      case "multi_select":
        next.controls[name] = {
          ...current,
          type: "multi_select",
          operator: current.operator || definition.operator || "in",
          values: definition.urlParam ? paramArray(params[definition.urlParam]).sort() : [...base.values ?? []]
        };
        break;
      case "text": {
        const value = definition.urlParam ? paramString(params[definition.urlParam]) : base.value ?? "";
        const operator = definition.operatorURLParam ? paramString(params[definition.operatorURLParam]) : "";
        next.controls[name] = {
          ...current,
          type: "text",
          operator: operator && (definition.operators ?? []).includes(operator) ? operator : base.operator,
          value
        };
        break;
      }
    }
  }
  return next;
}
function dateControlFromParams(definition, current, params) {
  const base = defaultControl(definition);
  const preset = definition.urlParam ? paramString(params[definition.urlParam]) : "";
  const from = definition.fromURLParam ? paramString(params[definition.fromURLParam]) : "";
  const to = definition.toURLParam ? paramString(params[definition.toURLParam]) : "";
  if (from || to) {
    return { ...current, type: "date_range", preset: "custom", from, to };
  }
  if (!preset) {
    return base;
  }
  if (preset === "custom") {
    return { ...current, type: "date_range", preset: "custom", from: "", to: "" };
  }
  if ((definition.presets ?? []).some((item) => item.value === preset)) {
    return { ...current, type: "date_range", preset, from: "", to: "" };
  }
  return base;
}
function paramString(value) {
  if (Array.isArray(value)) {
    return value[0] ?? "";
  }
  return (value ?? "").trim();
}
function paramArray(value) {
  const values = Array.isArray(value) ? value : value ? [value] : [];
  return [...new Set(values.map((item) => item.trim()).filter(Boolean))];
}

// web/components/url-sync.ts
var dataStarURLSyncEvent = "datastar-url-params-sync";
function normalizeURLParams(value) {
  const record = typeof value === "object" && value !== null ? value : {};
  const out = {};
  for (const [key, raw] of Object.entries(record)) {
    if (Array.isArray(raw)) {
      const seen = /* @__PURE__ */ new Set();
      out[key] = raw.flatMap((item) => {
        if (typeof item !== "string") return [];
        const trimmed = item.trim();
        if (!trimmed || seen.has(trimmed)) return [];
        seen.add(trimmed);
        return [trimmed];
      });
      continue;
    }
    out[key] = typeof raw === "string" ? raw.trim() : "";
  }
  return out;
}
function toQueryString(value) {
  const params = normalizeURLParams(value);
  const search = new URLSearchParams();
  for (const [key, raw] of Object.entries(params)) {
    if (Array.isArray(raw)) {
      for (const item of raw) search.append(key, item);
      continue;
    }
    if (raw) search.set(key, raw);
  }
  return search.toString();
}
function toURL(path, value) {
  const query = toQueryString(value);
  return query ? `${path}?${query}` : path;
}
function readLocation(shape) {
  const base = normalizeURLParams(shape);
  const url = new URL(window.location.href);
  const next = {};
  for (const [key, raw] of Object.entries(base)) {
    if (Array.isArray(raw)) {
      next[key] = url.searchParams.getAll(key).map((item) => item.trim()).filter(Boolean);
      continue;
    }
    next[key] = url.searchParams.get(key)?.trim() ?? raw;
  }
  return next;
}
function emit(params) {
  window.dispatchEvent(new CustomEvent(dataStarURLSyncEvent, {
    detail: {
      params,
      url: `${window.location.pathname}${window.location.search}`
    }
  }));
  return params;
}
function updateHistory(method, value, path = window.location.pathname) {
  const next = toURL(path, value);
  const current = `${window.location.pathname}${window.location.search}`;
  if (next !== current) {
    window.history[method]({}, "", next);
  }
  return next;
}
function replace(value, path = window.location.pathname) {
  return updateHistory("replaceState", value, path);
}
function push(value, path = window.location.pathname) {
  return updateHistory("pushState", value, path);
}
var popstateBound = false;
function bindPopstate(fallback) {
  if (popstateBound) return;
  popstateBound = true;
  window.addEventListener("popstate", () => {
    emit(readLocation(fallback));
  });
}
var datastarURLSync = {
  bindPopstate,
  push,
  replace
};
var libreDashFilterURL = {
  fromParams(config, filters, params) {
    return filtersFromURLParams(config, filters, params);
  }
};
window.DatastarURLSync = datastarURLSync;
window.LibreDashFilterURL = libreDashFilterURL;
