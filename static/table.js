// node_modules/@lit/reactive-element/css-tag.js
var t = globalThis;
var e = t.ShadowRoot && (void 0 === t.ShadyCSS || t.ShadyCSS.nativeShadow) && "adoptedStyleSheets" in Document.prototype && "replace" in CSSStyleSheet.prototype;
var s = /* @__PURE__ */ Symbol();
var o = /* @__PURE__ */ new WeakMap();
var n = class {
  constructor(t5, e6, o7) {
    if (this._$cssResult$ = true, o7 !== s) throw Error("CSSResult is not constructable. Use `unsafeCSS` or `css` instead.");
    this.cssText = t5, this.t = e6;
  }
  get styleSheet() {
    let t5 = this.o;
    const s5 = this.t;
    if (e && void 0 === t5) {
      const e6 = void 0 !== s5 && 1 === s5.length;
      e6 && (t5 = o.get(s5)), void 0 === t5 && ((this.o = t5 = new CSSStyleSheet()).replaceSync(this.cssText), e6 && o.set(s5, t5));
    }
    return t5;
  }
  toString() {
    return this.cssText;
  }
};
var r = (t5) => new n("string" == typeof t5 ? t5 : t5 + "", void 0, s);
var i = (t5, ...e6) => {
  const o7 = 1 === t5.length ? t5[0] : e6.reduce((e7, s5, o8) => e7 + ((t6) => {
    if (true === t6._$cssResult$) return t6.cssText;
    if ("number" == typeof t6) return t6;
    throw Error("Value passed to 'css' function must be a 'css' function result: " + t6 + ". Use 'unsafeCSS' to pass non-literal values, but take care to ensure page security.");
  })(s5) + t5[o8 + 1], t5[0]);
  return new n(o7, t5, s);
};
var S = (s5, o7) => {
  if (e) s5.adoptedStyleSheets = o7.map((t5) => t5 instanceof CSSStyleSheet ? t5 : t5.styleSheet);
  else for (const e6 of o7) {
    const o8 = document.createElement("style"), n6 = t.litNonce;
    void 0 !== n6 && o8.setAttribute("nonce", n6), o8.textContent = e6.cssText, s5.appendChild(o8);
  }
};
var c = e ? (t5) => t5 : (t5) => t5 instanceof CSSStyleSheet ? ((t6) => {
  let e6 = "";
  for (const s5 of t6.cssRules) e6 += s5.cssText;
  return r(e6);
})(t5) : t5;

// node_modules/@lit/reactive-element/reactive-element.js
var { is: i2, defineProperty: e2, getOwnPropertyDescriptor: h, getOwnPropertyNames: r2, getOwnPropertySymbols: o2, getPrototypeOf: n2 } = Object;
var a = globalThis;
var c2 = a.trustedTypes;
var l = c2 ? c2.emptyScript : "";
var p = a.reactiveElementPolyfillSupport;
var d = (t5, s5) => t5;
var u = { toAttribute(t5, s5) {
  switch (s5) {
    case Boolean:
      t5 = t5 ? l : null;
      break;
    case Object:
    case Array:
      t5 = null == t5 ? t5 : JSON.stringify(t5);
  }
  return t5;
}, fromAttribute(t5, s5) {
  let i6 = t5;
  switch (s5) {
    case Boolean:
      i6 = null !== t5;
      break;
    case Number:
      i6 = null === t5 ? null : Number(t5);
      break;
    case Object:
    case Array:
      try {
        i6 = JSON.parse(t5);
      } catch (t6) {
        i6 = null;
      }
  }
  return i6;
} };
var f = (t5, s5) => !i2(t5, s5);
var b = { attribute: true, type: String, converter: u, reflect: false, useDefault: false, hasChanged: f };
Symbol.metadata ??= /* @__PURE__ */ Symbol("metadata"), a.litPropertyMetadata ??= /* @__PURE__ */ new WeakMap();
var y = class extends HTMLElement {
  static addInitializer(t5) {
    this._$Ei(), (this.l ??= []).push(t5);
  }
  static get observedAttributes() {
    return this.finalize(), this._$Eh && [...this._$Eh.keys()];
  }
  static createProperty(t5, s5 = b) {
    if (s5.state && (s5.attribute = false), this._$Ei(), this.prototype.hasOwnProperty(t5) && ((s5 = Object.create(s5)).wrapped = true), this.elementProperties.set(t5, s5), !s5.noAccessor) {
      const i6 = /* @__PURE__ */ Symbol(), h5 = this.getPropertyDescriptor(t5, i6, s5);
      void 0 !== h5 && e2(this.prototype, t5, h5);
    }
  }
  static getPropertyDescriptor(t5, s5, i6) {
    const { get: e6, set: r6 } = h(this.prototype, t5) ?? { get() {
      return this[s5];
    }, set(t6) {
      this[s5] = t6;
    } };
    return { get: e6, set(s6) {
      const h5 = e6?.call(this);
      r6?.call(this, s6), this.requestUpdate(t5, h5, i6);
    }, configurable: true, enumerable: true };
  }
  static getPropertyOptions(t5) {
    return this.elementProperties.get(t5) ?? b;
  }
  static _$Ei() {
    if (this.hasOwnProperty(d("elementProperties"))) return;
    const t5 = n2(this);
    t5.finalize(), void 0 !== t5.l && (this.l = [...t5.l]), this.elementProperties = new Map(t5.elementProperties);
  }
  static finalize() {
    if (this.hasOwnProperty(d("finalized"))) return;
    if (this.finalized = true, this._$Ei(), this.hasOwnProperty(d("properties"))) {
      const t6 = this.properties, s5 = [...r2(t6), ...o2(t6)];
      for (const i6 of s5) this.createProperty(i6, t6[i6]);
    }
    const t5 = this[Symbol.metadata];
    if (null !== t5) {
      const s5 = litPropertyMetadata.get(t5);
      if (void 0 !== s5) for (const [t6, i6] of s5) this.elementProperties.set(t6, i6);
    }
    this._$Eh = /* @__PURE__ */ new Map();
    for (const [t6, s5] of this.elementProperties) {
      const i6 = this._$Eu(t6, s5);
      void 0 !== i6 && this._$Eh.set(i6, t6);
    }
    this.elementStyles = this.finalizeStyles(this.styles);
  }
  static finalizeStyles(s5) {
    const i6 = [];
    if (Array.isArray(s5)) {
      const e6 = new Set(s5.flat(1 / 0).reverse());
      for (const s6 of e6) i6.unshift(c(s6));
    } else void 0 !== s5 && i6.push(c(s5));
    return i6;
  }
  static _$Eu(t5, s5) {
    const i6 = s5.attribute;
    return false === i6 ? void 0 : "string" == typeof i6 ? i6 : "string" == typeof t5 ? t5.toLowerCase() : void 0;
  }
  constructor() {
    super(), this._$Ep = void 0, this.isUpdatePending = false, this.hasUpdated = false, this._$Em = null, this._$Ev();
  }
  _$Ev() {
    this._$ES = new Promise((t5) => this.enableUpdating = t5), this._$AL = /* @__PURE__ */ new Map(), this._$E_(), this.requestUpdate(), this.constructor.l?.forEach((t5) => t5(this));
  }
  addController(t5) {
    (this._$EO ??= /* @__PURE__ */ new Set()).add(t5), void 0 !== this.renderRoot && this.isConnected && t5.hostConnected?.();
  }
  removeController(t5) {
    this._$EO?.delete(t5);
  }
  _$E_() {
    const t5 = /* @__PURE__ */ new Map(), s5 = this.constructor.elementProperties;
    for (const i6 of s5.keys()) this.hasOwnProperty(i6) && (t5.set(i6, this[i6]), delete this[i6]);
    t5.size > 0 && (this._$Ep = t5);
  }
  createRenderRoot() {
    const t5 = this.shadowRoot ?? this.attachShadow(this.constructor.shadowRootOptions);
    return S(t5, this.constructor.elementStyles), t5;
  }
  connectedCallback() {
    this.renderRoot ??= this.createRenderRoot(), this.enableUpdating(true), this._$EO?.forEach((t5) => t5.hostConnected?.());
  }
  enableUpdating(t5) {
  }
  disconnectedCallback() {
    this._$EO?.forEach((t5) => t5.hostDisconnected?.());
  }
  attributeChangedCallback(t5, s5, i6) {
    this._$AK(t5, i6);
  }
  _$ET(t5, s5) {
    const i6 = this.constructor.elementProperties.get(t5), e6 = this.constructor._$Eu(t5, i6);
    if (void 0 !== e6 && true === i6.reflect) {
      const h5 = (void 0 !== i6.converter?.toAttribute ? i6.converter : u).toAttribute(s5, i6.type);
      this._$Em = t5, null == h5 ? this.removeAttribute(e6) : this.setAttribute(e6, h5), this._$Em = null;
    }
  }
  _$AK(t5, s5) {
    const i6 = this.constructor, e6 = i6._$Eh.get(t5);
    if (void 0 !== e6 && this._$Em !== e6) {
      const t6 = i6.getPropertyOptions(e6), h5 = "function" == typeof t6.converter ? { fromAttribute: t6.converter } : void 0 !== t6.converter?.fromAttribute ? t6.converter : u;
      this._$Em = e6;
      const r6 = h5.fromAttribute(s5, t6.type);
      this[e6] = r6 ?? this._$Ej?.get(e6) ?? r6, this._$Em = null;
    }
  }
  requestUpdate(t5, s5, i6, e6 = false, h5) {
    if (void 0 !== t5) {
      const r6 = this.constructor;
      if (false === e6 && (h5 = this[t5]), i6 ??= r6.getPropertyOptions(t5), !((i6.hasChanged ?? f)(h5, s5) || i6.useDefault && i6.reflect && h5 === this._$Ej?.get(t5) && !this.hasAttribute(r6._$Eu(t5, i6)))) return;
      this.C(t5, s5, i6);
    }
    false === this.isUpdatePending && (this._$ES = this._$EP());
  }
  C(t5, s5, { useDefault: i6, reflect: e6, wrapped: h5 }, r6) {
    i6 && !(this._$Ej ??= /* @__PURE__ */ new Map()).has(t5) && (this._$Ej.set(t5, r6 ?? s5 ?? this[t5]), true !== h5 || void 0 !== r6) || (this._$AL.has(t5) || (this.hasUpdated || i6 || (s5 = void 0), this._$AL.set(t5, s5)), true === e6 && this._$Em !== t5 && (this._$Eq ??= /* @__PURE__ */ new Set()).add(t5));
  }
  async _$EP() {
    this.isUpdatePending = true;
    try {
      await this._$ES;
    } catch (t6) {
      Promise.reject(t6);
    }
    const t5 = this.scheduleUpdate();
    return null != t5 && await t5, !this.isUpdatePending;
  }
  scheduleUpdate() {
    return this.performUpdate();
  }
  performUpdate() {
    if (!this.isUpdatePending) return;
    if (!this.hasUpdated) {
      if (this.renderRoot ??= this.createRenderRoot(), this._$Ep) {
        for (const [t7, s6] of this._$Ep) this[t7] = s6;
        this._$Ep = void 0;
      }
      const t6 = this.constructor.elementProperties;
      if (t6.size > 0) for (const [s6, i6] of t6) {
        const { wrapped: t7 } = i6, e6 = this[s6];
        true !== t7 || this._$AL.has(s6) || void 0 === e6 || this.C(s6, void 0, i6, e6);
      }
    }
    let t5 = false;
    const s5 = this._$AL;
    try {
      t5 = this.shouldUpdate(s5), t5 ? (this.willUpdate(s5), this._$EO?.forEach((t6) => t6.hostUpdate?.()), this.update(s5)) : this._$EM();
    } catch (s6) {
      throw t5 = false, this._$EM(), s6;
    }
    t5 && this._$AE(s5);
  }
  willUpdate(t5) {
  }
  _$AE(t5) {
    this._$EO?.forEach((t6) => t6.hostUpdated?.()), this.hasUpdated || (this.hasUpdated = true, this.firstUpdated(t5)), this.updated(t5);
  }
  _$EM() {
    this._$AL = /* @__PURE__ */ new Map(), this.isUpdatePending = false;
  }
  get updateComplete() {
    return this.getUpdateComplete();
  }
  getUpdateComplete() {
    return this._$ES;
  }
  shouldUpdate(t5) {
    return true;
  }
  update(t5) {
    this._$Eq &&= this._$Eq.forEach((t6) => this._$ET(t6, this[t6])), this._$EM();
  }
  updated(t5) {
  }
  firstUpdated(t5) {
  }
};
y.elementStyles = [], y.shadowRootOptions = { mode: "open" }, y[d("elementProperties")] = /* @__PURE__ */ new Map(), y[d("finalized")] = /* @__PURE__ */ new Map(), p?.({ ReactiveElement: y }), (a.reactiveElementVersions ??= []).push("2.1.2");

// node_modules/lit-html/lit-html.js
var t2 = globalThis;
var i3 = (t5) => t5;
var s2 = t2.trustedTypes;
var e3 = s2 ? s2.createPolicy("lit-html", { createHTML: (t5) => t5 }) : void 0;
var h2 = "$lit$";
var o3 = `lit$${Math.random().toFixed(9).slice(2)}$`;
var n3 = "?" + o3;
var r3 = `<${n3}>`;
var l2 = document;
var c3 = () => l2.createComment("");
var a2 = (t5) => null === t5 || "object" != typeof t5 && "function" != typeof t5;
var u2 = Array.isArray;
var d2 = (t5) => u2(t5) || "function" == typeof t5?.[Symbol.iterator];
var f2 = "[ 	\n\f\r]";
var v = /<(?:(!--|\/[^a-zA-Z])|(\/?[a-zA-Z][^>\s]*)|(\/?$))/g;
var _ = /-->/g;
var m = />/g;
var p2 = RegExp(`>|${f2}(?:([^\\s"'>=/]+)(${f2}*=${f2}*(?:[^ 	
\f\r"'\`<>=]|("|')|))|$)`, "g");
var g = /'/g;
var $ = /"/g;
var y2 = /^(?:script|style|textarea|title)$/i;
var x = (t5) => (i6, ...s5) => ({ _$litType$: t5, strings: i6, values: s5 });
var b2 = x(1);
var w = x(2);
var T = x(3);
var E = /* @__PURE__ */ Symbol.for("lit-noChange");
var A = /* @__PURE__ */ Symbol.for("lit-nothing");
var C = /* @__PURE__ */ new WeakMap();
var P = l2.createTreeWalker(l2, 129);
function V(t5, i6) {
  if (!u2(t5) || !t5.hasOwnProperty("raw")) throw Error("invalid template strings array");
  return void 0 !== e3 ? e3.createHTML(i6) : i6;
}
var N = (t5, i6) => {
  const s5 = t5.length - 1, e6 = [];
  let n6, l3 = 2 === i6 ? "<svg>" : 3 === i6 ? "<math>" : "", c5 = v;
  for (let i7 = 0; i7 < s5; i7++) {
    const s6 = t5[i7];
    let a3, u3, d3 = -1, f4 = 0;
    for (; f4 < s6.length && (c5.lastIndex = f4, u3 = c5.exec(s6), null !== u3); ) f4 = c5.lastIndex, c5 === v ? "!--" === u3[1] ? c5 = _ : void 0 !== u3[1] ? c5 = m : void 0 !== u3[2] ? (y2.test(u3[2]) && (n6 = RegExp("</" + u3[2], "g")), c5 = p2) : void 0 !== u3[3] && (c5 = p2) : c5 === p2 ? ">" === u3[0] ? (c5 = n6 ?? v, d3 = -1) : void 0 === u3[1] ? d3 = -2 : (d3 = c5.lastIndex - u3[2].length, a3 = u3[1], c5 = void 0 === u3[3] ? p2 : '"' === u3[3] ? $ : g) : c5 === $ || c5 === g ? c5 = p2 : c5 === _ || c5 === m ? c5 = v : (c5 = p2, n6 = void 0);
    const x2 = c5 === p2 && t5[i7 + 1].startsWith("/>") ? " " : "";
    l3 += c5 === v ? s6 + r3 : d3 >= 0 ? (e6.push(a3), s6.slice(0, d3) + h2 + s6.slice(d3) + o3 + x2) : s6 + o3 + (-2 === d3 ? i7 : x2);
  }
  return [V(t5, l3 + (t5[s5] || "<?>") + (2 === i6 ? "</svg>" : 3 === i6 ? "</math>" : "")), e6];
};
var S2 = class _S {
  constructor({ strings: t5, _$litType$: i6 }, e6) {
    let r6;
    this.parts = [];
    let l3 = 0, a3 = 0;
    const u3 = t5.length - 1, d3 = this.parts, [f4, v2] = N(t5, i6);
    if (this.el = _S.createElement(f4, e6), P.currentNode = this.el.content, 2 === i6 || 3 === i6) {
      const t6 = this.el.content.firstChild;
      t6.replaceWith(...t6.childNodes);
    }
    for (; null !== (r6 = P.nextNode()) && d3.length < u3; ) {
      if (1 === r6.nodeType) {
        if (r6.hasAttributes()) for (const t6 of r6.getAttributeNames()) if (t6.endsWith(h2)) {
          const i7 = v2[a3++], s5 = r6.getAttribute(t6).split(o3), e7 = /([.?@])?(.*)/.exec(i7);
          d3.push({ type: 1, index: l3, name: e7[2], strings: s5, ctor: "." === e7[1] ? I : "?" === e7[1] ? L : "@" === e7[1] ? z : H }), r6.removeAttribute(t6);
        } else t6.startsWith(o3) && (d3.push({ type: 6, index: l3 }), r6.removeAttribute(t6));
        if (y2.test(r6.tagName)) {
          const t6 = r6.textContent.split(o3), i7 = t6.length - 1;
          if (i7 > 0) {
            r6.textContent = s2 ? s2.emptyScript : "";
            for (let s5 = 0; s5 < i7; s5++) r6.append(t6[s5], c3()), P.nextNode(), d3.push({ type: 2, index: ++l3 });
            r6.append(t6[i7], c3());
          }
        }
      } else if (8 === r6.nodeType) if (r6.data === n3) d3.push({ type: 2, index: l3 });
      else {
        let t6 = -1;
        for (; -1 !== (t6 = r6.data.indexOf(o3, t6 + 1)); ) d3.push({ type: 7, index: l3 }), t6 += o3.length - 1;
      }
      l3++;
    }
  }
  static createElement(t5, i6) {
    const s5 = l2.createElement("template");
    return s5.innerHTML = t5, s5;
  }
};
function M(t5, i6, s5 = t5, e6) {
  if (i6 === E) return i6;
  let h5 = void 0 !== e6 ? s5._$Co?.[e6] : s5._$Cl;
  const o7 = a2(i6) ? void 0 : i6._$litDirective$;
  return h5?.constructor !== o7 && (h5?._$AO?.(false), void 0 === o7 ? h5 = void 0 : (h5 = new o7(t5), h5._$AT(t5, s5, e6)), void 0 !== e6 ? (s5._$Co ??= [])[e6] = h5 : s5._$Cl = h5), void 0 !== h5 && (i6 = M(t5, h5._$AS(t5, i6.values), h5, e6)), i6;
}
var R = class {
  constructor(t5, i6) {
    this._$AV = [], this._$AN = void 0, this._$AD = t5, this._$AM = i6;
  }
  get parentNode() {
    return this._$AM.parentNode;
  }
  get _$AU() {
    return this._$AM._$AU;
  }
  u(t5) {
    const { el: { content: i6 }, parts: s5 } = this._$AD, e6 = (t5?.creationScope ?? l2).importNode(i6, true);
    P.currentNode = e6;
    let h5 = P.nextNode(), o7 = 0, n6 = 0, r6 = s5[0];
    for (; void 0 !== r6; ) {
      if (o7 === r6.index) {
        let i7;
        2 === r6.type ? i7 = new k(h5, h5.nextSibling, this, t5) : 1 === r6.type ? i7 = new r6.ctor(h5, r6.name, r6.strings, this, t5) : 6 === r6.type && (i7 = new Z(h5, this, t5)), this._$AV.push(i7), r6 = s5[++n6];
      }
      o7 !== r6?.index && (h5 = P.nextNode(), o7++);
    }
    return P.currentNode = l2, e6;
  }
  p(t5) {
    let i6 = 0;
    for (const s5 of this._$AV) void 0 !== s5 && (void 0 !== s5.strings ? (s5._$AI(t5, s5, i6), i6 += s5.strings.length - 2) : s5._$AI(t5[i6])), i6++;
  }
};
var k = class _k {
  get _$AU() {
    return this._$AM?._$AU ?? this._$Cv;
  }
  constructor(t5, i6, s5, e6) {
    this.type = 2, this._$AH = A, this._$AN = void 0, this._$AA = t5, this._$AB = i6, this._$AM = s5, this.options = e6, this._$Cv = e6?.isConnected ?? true;
  }
  get parentNode() {
    let t5 = this._$AA.parentNode;
    const i6 = this._$AM;
    return void 0 !== i6 && 11 === t5?.nodeType && (t5 = i6.parentNode), t5;
  }
  get startNode() {
    return this._$AA;
  }
  get endNode() {
    return this._$AB;
  }
  _$AI(t5, i6 = this) {
    t5 = M(this, t5, i6), a2(t5) ? t5 === A || null == t5 || "" === t5 ? (this._$AH !== A && this._$AR(), this._$AH = A) : t5 !== this._$AH && t5 !== E && this._(t5) : void 0 !== t5._$litType$ ? this.$(t5) : void 0 !== t5.nodeType ? this.T(t5) : d2(t5) ? this.k(t5) : this._(t5);
  }
  O(t5) {
    return this._$AA.parentNode.insertBefore(t5, this._$AB);
  }
  T(t5) {
    this._$AH !== t5 && (this._$AR(), this._$AH = this.O(t5));
  }
  _(t5) {
    this._$AH !== A && a2(this._$AH) ? this._$AA.nextSibling.data = t5 : this.T(l2.createTextNode(t5)), this._$AH = t5;
  }
  $(t5) {
    const { values: i6, _$litType$: s5 } = t5, e6 = "number" == typeof s5 ? this._$AC(t5) : (void 0 === s5.el && (s5.el = S2.createElement(V(s5.h, s5.h[0]), this.options)), s5);
    if (this._$AH?._$AD === e6) this._$AH.p(i6);
    else {
      const t6 = new R(e6, this), s6 = t6.u(this.options);
      t6.p(i6), this.T(s6), this._$AH = t6;
    }
  }
  _$AC(t5) {
    let i6 = C.get(t5.strings);
    return void 0 === i6 && C.set(t5.strings, i6 = new S2(t5)), i6;
  }
  k(t5) {
    u2(this._$AH) || (this._$AH = [], this._$AR());
    const i6 = this._$AH;
    let s5, e6 = 0;
    for (const h5 of t5) e6 === i6.length ? i6.push(s5 = new _k(this.O(c3()), this.O(c3()), this, this.options)) : s5 = i6[e6], s5._$AI(h5), e6++;
    e6 < i6.length && (this._$AR(s5 && s5._$AB.nextSibling, e6), i6.length = e6);
  }
  _$AR(t5 = this._$AA.nextSibling, s5) {
    for (this._$AP?.(false, true, s5); t5 !== this._$AB; ) {
      const s6 = i3(t5).nextSibling;
      i3(t5).remove(), t5 = s6;
    }
  }
  setConnected(t5) {
    void 0 === this._$AM && (this._$Cv = t5, this._$AP?.(t5));
  }
};
var H = class {
  get tagName() {
    return this.element.tagName;
  }
  get _$AU() {
    return this._$AM._$AU;
  }
  constructor(t5, i6, s5, e6, h5) {
    this.type = 1, this._$AH = A, this._$AN = void 0, this.element = t5, this.name = i6, this._$AM = e6, this.options = h5, s5.length > 2 || "" !== s5[0] || "" !== s5[1] ? (this._$AH = Array(s5.length - 1).fill(new String()), this.strings = s5) : this._$AH = A;
  }
  _$AI(t5, i6 = this, s5, e6) {
    const h5 = this.strings;
    let o7 = false;
    if (void 0 === h5) t5 = M(this, t5, i6, 0), o7 = !a2(t5) || t5 !== this._$AH && t5 !== E, o7 && (this._$AH = t5);
    else {
      const e7 = t5;
      let n6, r6;
      for (t5 = h5[0], n6 = 0; n6 < h5.length - 1; n6++) r6 = M(this, e7[s5 + n6], i6, n6), r6 === E && (r6 = this._$AH[n6]), o7 ||= !a2(r6) || r6 !== this._$AH[n6], r6 === A ? t5 = A : t5 !== A && (t5 += (r6 ?? "") + h5[n6 + 1]), this._$AH[n6] = r6;
    }
    o7 && !e6 && this.j(t5);
  }
  j(t5) {
    t5 === A ? this.element.removeAttribute(this.name) : this.element.setAttribute(this.name, t5 ?? "");
  }
};
var I = class extends H {
  constructor() {
    super(...arguments), this.type = 3;
  }
  j(t5) {
    this.element[this.name] = t5 === A ? void 0 : t5;
  }
};
var L = class extends H {
  constructor() {
    super(...arguments), this.type = 4;
  }
  j(t5) {
    this.element.toggleAttribute(this.name, !!t5 && t5 !== A);
  }
};
var z = class extends H {
  constructor(t5, i6, s5, e6, h5) {
    super(t5, i6, s5, e6, h5), this.type = 5;
  }
  _$AI(t5, i6 = this) {
    if ((t5 = M(this, t5, i6, 0) ?? A) === E) return;
    const s5 = this._$AH, e6 = t5 === A && s5 !== A || t5.capture !== s5.capture || t5.once !== s5.once || t5.passive !== s5.passive, h5 = t5 !== A && (s5 === A || e6);
    e6 && this.element.removeEventListener(this.name, this, s5), h5 && this.element.addEventListener(this.name, this, t5), this._$AH = t5;
  }
  handleEvent(t5) {
    "function" == typeof this._$AH ? this._$AH.call(this.options?.host ?? this.element, t5) : this._$AH.handleEvent(t5);
  }
};
var Z = class {
  constructor(t5, i6, s5) {
    this.element = t5, this.type = 6, this._$AN = void 0, this._$AM = i6, this.options = s5;
  }
  get _$AU() {
    return this._$AM._$AU;
  }
  _$AI(t5) {
    M(this, t5);
  }
};
var j = { M: h2, P: o3, A: n3, C: 1, L: N, R, D: d2, V: M, I: k, H, N: L, U: z, B: I, F: Z };
var B = t2.litHtmlPolyfillSupport;
B?.(S2, k), (t2.litHtmlVersions ??= []).push("3.3.3");
var D = (t5, i6, s5) => {
  const e6 = s5?.renderBefore ?? i6;
  let h5 = e6._$litPart$;
  if (void 0 === h5) {
    const t6 = s5?.renderBefore ?? null;
    e6._$litPart$ = h5 = new k(i6.insertBefore(c3(), t6), t6, void 0, s5 ?? {});
  }
  return h5._$AI(t5), h5;
};

// node_modules/lit-element/lit-element.js
var s3 = globalThis;
var i4 = class extends y {
  constructor() {
    super(...arguments), this.renderOptions = { host: this }, this._$Do = void 0;
  }
  createRenderRoot() {
    const t5 = super.createRenderRoot();
    return this.renderOptions.renderBefore ??= t5.firstChild, t5;
  }
  update(t5) {
    const r6 = this.render();
    this.hasUpdated || (this.renderOptions.isConnected = this.isConnected), super.update(t5), this._$Do = D(r6, this.renderRoot, this.renderOptions);
  }
  connectedCallback() {
    super.connectedCallback(), this._$Do?.setConnected(true);
  }
  disconnectedCallback() {
    super.disconnectedCallback(), this._$Do?.setConnected(false);
  }
  render() {
    return E;
  }
};
i4._$litElement$ = true, i4["finalized"] = true, s3.litElementHydrateSupport?.({ LitElement: i4 });
var o4 = s3.litElementPolyfillSupport;
o4?.({ LitElement: i4 });
(s3.litElementVersions ??= []).push("4.2.2");

// node_modules/lit-html/directive-helpers.js
var { I: t3 } = j;
var r4 = (o7) => void 0 === o7.strings;

// node_modules/lit-html/directive.js
var t4 = { ATTRIBUTE: 1, CHILD: 2, PROPERTY: 3, BOOLEAN_ATTRIBUTE: 4, EVENT: 5, ELEMENT: 6 };
var e4 = (t5) => (...e6) => ({ _$litDirective$: t5, values: e6 });
var i5 = class {
  constructor(t5) {
  }
  get _$AU() {
    return this._$AM._$AU;
  }
  _$AT(t5, e6, i6) {
    this._$Ct = t5, this._$AM = e6, this._$Ci = i6;
  }
  _$AS(t5, e6) {
    return this.update(t5, e6);
  }
  update(t5, e6) {
    return this.render(...e6);
  }
};

// node_modules/lit-html/async-directive.js
var s4 = (i6, t5) => {
  const e6 = i6._$AN;
  if (void 0 === e6) return false;
  for (const i7 of e6) i7._$AO?.(t5, false), s4(i7, t5);
  return true;
};
var o5 = (i6) => {
  let t5, e6;
  do {
    if (void 0 === (t5 = i6._$AM)) break;
    e6 = t5._$AN, e6.delete(i6), i6 = t5;
  } while (0 === e6?.size);
};
var r5 = (i6) => {
  for (let t5; t5 = i6._$AM; i6 = t5) {
    let e6 = t5._$AN;
    if (void 0 === e6) t5._$AN = e6 = /* @__PURE__ */ new Set();
    else if (e6.has(i6)) break;
    e6.add(i6), c4(t5);
  }
};
function h3(i6) {
  void 0 !== this._$AN ? (o5(this), this._$AM = i6, r5(this)) : this._$AM = i6;
}
function n4(i6, t5 = false, e6 = 0) {
  const r6 = this._$AH, h5 = this._$AN;
  if (void 0 !== h5 && 0 !== h5.size) if (t5) if (Array.isArray(r6)) for (let i7 = e6; i7 < r6.length; i7++) s4(r6[i7], false), o5(r6[i7]);
  else null != r6 && (s4(r6, false), o5(r6));
  else s4(this, i6);
}
var c4 = (i6) => {
  i6.type == t4.CHILD && (i6._$AP ??= n4, i6._$AQ ??= h3);
};
var f3 = class extends i5 {
  constructor() {
    super(...arguments), this._$AN = void 0;
  }
  _$AT(i6, t5, e6) {
    super._$AT(i6, t5, e6), r5(this), this.isConnected = i6._$AU;
  }
  _$AO(i6, t5 = true) {
    i6 !== this.isConnected && (this.isConnected = i6, i6 ? this.reconnected?.() : this.disconnected?.()), t5 && (s4(this, i6), o5(this));
  }
  setValue(t5) {
    if (r4(this._$Ct)) this._$Ct._$AI(t5, this);
    else {
      const i6 = [...this._$Ct._$AH];
      i6[this._$Ci] = t5, this._$Ct._$AI(i6, this, 0);
    }
  }
  disconnected() {
  }
  reconnected() {
  }
};

// node_modules/lit-html/directives/ref.js
var e5 = () => new h4();
var h4 = class {
};
var o6 = /* @__PURE__ */ new WeakMap();
var n5 = e4(class extends f3 {
  render(i6) {
    return A;
  }
  update(i6, [s5]) {
    const e6 = s5 !== this.G;
    return e6 && this.rt(void 0), (e6 || this.lt !== this.ct) && (this.G = s5, this.ht = i6.options?.host, this.rt(this.ct = i6.element)), A;
  }
  rt(t5) {
    if (void 0 !== this.G) if (this.isConnected || (t5 = void 0), "function" == typeof this.G) {
      const i6 = this.ht ?? globalThis;
      let s5 = o6.get(i6);
      void 0 === s5 && (s5 = /* @__PURE__ */ new WeakMap(), o6.set(i6, s5)), void 0 !== s5.get(this.G) && this.G.call(this.ht, void 0), s5.set(this.G, t5), void 0 !== t5 && this.G.call(this.ht, t5);
    } else this.G.value = t5;
  }
  get lt() {
    return "function" == typeof this.G ? o6.get(this.ht ?? globalThis)?.get(this.G) : this.G?.value;
  }
  disconnected() {
    this.lt === this.ct && this.rt(void 0);
  }
  reconnected() {
    this.rt(this.ct);
  }
});

// node_modules/@tanstack/lit-table/dist/flexRender.js
function flexRender(Comp, props) {
  if (!Comp) return null;
  if (typeof Comp === "function") return Comp(props);
  return Comp;
}
function FlexRender(props) {
  if ("cell" in props && props.cell) return flexRender(props.cell.column.columnDef.cell, props.cell.getContext());
  if ("header" in props && props.header) return flexRender(props.header.column.columnDef.header, props.header.getContext());
  if ("footer" in props && props.footer) return flexRender(props.footer.column.columnDef.footer, props.footer.getContext());
  return null;
}

// node_modules/@tanstack/store/dist/alien.js
var ReactiveFlags = /* @__PURE__ */ (function(ReactiveFlags2) {
  ReactiveFlags2[ReactiveFlags2["None"] = 0] = "None";
  ReactiveFlags2[ReactiveFlags2["Mutable"] = 1] = "Mutable";
  ReactiveFlags2[ReactiveFlags2["Watching"] = 2] = "Watching";
  ReactiveFlags2[ReactiveFlags2["RecursedCheck"] = 4] = "RecursedCheck";
  ReactiveFlags2[ReactiveFlags2["Recursed"] = 8] = "Recursed";
  ReactiveFlags2[ReactiveFlags2["Dirty"] = 16] = "Dirty";
  ReactiveFlags2[ReactiveFlags2["Pending"] = 32] = "Pending";
  return ReactiveFlags2;
})({});
// @__NO_SIDE_EFFECTS__
function createReactiveSystem({ update, notify, unwatched }) {
  return {
    link: link2,
    unlink: unlink2,
    propagate: propagate2,
    checkDirty: checkDirty2,
    shallowPropagate: shallowPropagate2
  };
  function link2(dep, sub, version) {
    const prevDep = sub.depsTail;
    if (prevDep !== void 0 && prevDep.dep === dep) return;
    const nextDep = prevDep !== void 0 ? prevDep.nextDep : sub.deps;
    if (nextDep !== void 0 && nextDep.dep === dep) {
      nextDep.version = version;
      sub.depsTail = nextDep;
      return;
    }
    const prevSub = dep.subsTail;
    if (prevSub !== void 0 && prevSub.version === version && prevSub.sub === sub) return;
    const newLink = sub.depsTail = dep.subsTail = {
      version,
      dep,
      sub,
      prevDep,
      nextDep,
      prevSub,
      nextSub: void 0
    };
    if (nextDep !== void 0) nextDep.prevDep = newLink;
    if (prevDep !== void 0) prevDep.nextDep = newLink;
    else sub.deps = newLink;
    if (prevSub !== void 0) prevSub.nextSub = newLink;
    else dep.subs = newLink;
  }
  function unlink2(link3, sub = link3.sub) {
    const dep = link3.dep;
    const prevDep = link3.prevDep;
    const nextDep = link3.nextDep;
    const nextSub = link3.nextSub;
    const prevSub = link3.prevSub;
    if (nextDep !== void 0) nextDep.prevDep = prevDep;
    else sub.depsTail = prevDep;
    if (prevDep !== void 0) prevDep.nextDep = nextDep;
    else sub.deps = nextDep;
    if (nextSub !== void 0) nextSub.prevSub = prevSub;
    else dep.subsTail = prevSub;
    if (prevSub !== void 0) prevSub.nextSub = nextSub;
    else if ((dep.subs = nextSub) === void 0) unwatched(dep);
    return nextDep;
  }
  function propagate2(link3) {
    let next = link3.nextSub;
    let stack;
    top: do {
      const sub = link3.sub;
      let flags = sub.flags;
      if (!(flags & (ReactiveFlags.RecursedCheck | ReactiveFlags.Recursed | ReactiveFlags.Dirty | ReactiveFlags.Pending))) sub.flags = flags | ReactiveFlags.Pending;
      else if (!(flags & (ReactiveFlags.RecursedCheck | ReactiveFlags.Recursed))) flags = ReactiveFlags.None;
      else if (!(flags & ReactiveFlags.RecursedCheck)) sub.flags = flags & ~ReactiveFlags.Recursed | ReactiveFlags.Pending;
      else if (!(flags & (ReactiveFlags.Dirty | ReactiveFlags.Pending)) && isValidLink(link3, sub)) {
        sub.flags = flags | (ReactiveFlags.Recursed | ReactiveFlags.Pending);
        flags &= ReactiveFlags.Mutable;
      } else flags = ReactiveFlags.None;
      if (flags & ReactiveFlags.Watching) notify(sub);
      if (flags & ReactiveFlags.Mutable) {
        const subSubs = sub.subs;
        if (subSubs !== void 0) {
          const nextSub = (link3 = subSubs).nextSub;
          if (nextSub !== void 0) {
            stack = {
              value: next,
              prev: stack
            };
            next = nextSub;
          }
          continue;
        }
      }
      if ((link3 = next) !== void 0) {
        next = link3.nextSub;
        continue;
      }
      while (stack !== void 0) {
        link3 = stack.value;
        stack = stack.prev;
        if (link3 !== void 0) {
          next = link3.nextSub;
          continue top;
        }
      }
      break;
    } while (true);
  }
  function checkDirty2(link3, sub) {
    let stack;
    let checkDepth = 0;
    let dirty = false;
    top: do {
      const dep = link3.dep;
      const flags = dep.flags;
      if (sub.flags & ReactiveFlags.Dirty) dirty = true;
      else if ((flags & (ReactiveFlags.Mutable | ReactiveFlags.Dirty)) === (ReactiveFlags.Mutable | ReactiveFlags.Dirty)) {
        if (update(dep)) {
          const subs = dep.subs;
          if (subs.nextSub !== void 0) shallowPropagate2(subs);
          dirty = true;
        }
      } else if ((flags & (ReactiveFlags.Mutable | ReactiveFlags.Pending)) === (ReactiveFlags.Mutable | ReactiveFlags.Pending)) {
        if (link3.nextSub !== void 0 || link3.prevSub !== void 0) stack = {
          value: link3,
          prev: stack
        };
        link3 = dep.deps;
        sub = dep;
        ++checkDepth;
        continue;
      }
      if (!dirty) {
        const nextDep = link3.nextDep;
        if (nextDep !== void 0) {
          link3 = nextDep;
          continue;
        }
      }
      while (checkDepth--) {
        const firstSub = sub.subs;
        const hasMultipleSubs = firstSub.nextSub !== void 0;
        if (hasMultipleSubs) {
          link3 = stack.value;
          stack = stack.prev;
        } else link3 = firstSub;
        if (dirty) {
          if (update(sub)) {
            if (hasMultipleSubs) shallowPropagate2(firstSub);
            sub = link3.sub;
            continue;
          }
          dirty = false;
        } else sub.flags &= ~ReactiveFlags.Pending;
        sub = link3.sub;
        const nextDep = link3.nextDep;
        if (nextDep !== void 0) {
          link3 = nextDep;
          continue top;
        }
      }
      return dirty;
    } while (true);
  }
  function shallowPropagate2(link3) {
    do {
      const sub = link3.sub;
      const flags = sub.flags;
      if ((flags & (ReactiveFlags.Pending | ReactiveFlags.Dirty)) === ReactiveFlags.Pending) {
        sub.flags = flags | ReactiveFlags.Dirty;
        if ((flags & (ReactiveFlags.Watching | ReactiveFlags.RecursedCheck)) === ReactiveFlags.Watching) notify(sub);
      }
    } while ((link3 = link3.nextSub) !== void 0);
  }
  function isValidLink(checkLink, sub) {
    let link3 = sub.depsTail;
    while (link3 !== void 0) {
      if (link3 === checkLink) return true;
      link3 = link3.prevDep;
    }
    return false;
  }
}

// node_modules/@tanstack/store/dist/atom.js
function toObserver(nextHandler, errorHandler, completionHandler) {
  const isObserver = typeof nextHandler === "object";
  const self = isObserver ? nextHandler : void 0;
  return {
    next: (isObserver ? nextHandler.next : nextHandler)?.bind(self),
    error: (isObserver ? nextHandler.error : errorHandler)?.bind(self),
    complete: (isObserver ? nextHandler.complete : completionHandler)?.bind(self)
  };
}
var queuedEffects = [];
var cycle = 0;
var { link, unlink, propagate, checkDirty, shallowPropagate } = /* @__PURE__ */ createReactiveSystem({
  update(atom) {
    return atom._update();
  },
  notify(effect2) {
    queuedEffects[queuedEffectsLength++] = effect2;
    effect2.flags &= ~ReactiveFlags.Watching;
  },
  unwatched(atom) {
    if (atom.depsTail !== void 0) {
      atom.depsTail = void 0;
      atom.flags = ReactiveFlags.Mutable | ReactiveFlags.Dirty;
      purgeDeps(atom);
    }
  }
});
var notifyIndex = 0;
var queuedEffectsLength = 0;
var activeSub;
var batchDepth = 0;
function batch(fn) {
  try {
    ++batchDepth;
    fn();
  } finally {
    if (!--batchDepth) flush();
  }
}
function purgeDeps(sub) {
  const depsTail = sub.depsTail;
  let dep = depsTail !== void 0 ? depsTail.nextDep : sub.deps;
  while (dep !== void 0) dep = unlink(dep, sub);
}
function flush() {
  if (batchDepth > 0) return;
  while (notifyIndex < queuedEffectsLength) {
    const effect2 = queuedEffects[notifyIndex];
    queuedEffects[notifyIndex++] = void 0;
    effect2.notify();
  }
  notifyIndex = 0;
  queuedEffectsLength = 0;
}
function createAtom(valueOrFn, options) {
  const isComputed = typeof valueOrFn === "function";
  const getter = valueOrFn;
  const atom = {
    _snapshot: isComputed ? void 0 : valueOrFn,
    subs: void 0,
    subsTail: void 0,
    deps: void 0,
    depsTail: void 0,
    flags: isComputed ? ReactiveFlags.None : ReactiveFlags.Mutable,
    get() {
      if (activeSub !== void 0) link(atom, activeSub, cycle);
      return atom._snapshot;
    },
    subscribe(observerOrFn) {
      const obs = toObserver(observerOrFn);
      const observed = { current: false };
      const e6 = effect(() => {
        atom.get();
        if (!observed.current) observed.current = true;
        else obs.next?.(atom._snapshot);
      });
      return { unsubscribe: () => {
        e6.stop();
      } };
    },
    _update(getValue) {
      const prevSub = activeSub;
      const compare = options?.compare ?? Object.is;
      if (isComputed) {
        activeSub = atom;
        ++cycle;
        atom.depsTail = void 0;
      } else if (getValue === void 0) return false;
      if (isComputed) atom.flags = ReactiveFlags.Mutable | ReactiveFlags.RecursedCheck;
      try {
        const oldValue = atom._snapshot;
        const newValue = typeof getValue === "function" ? getValue(oldValue) : getValue === void 0 && isComputed ? getter(oldValue) : getValue;
        if (oldValue === void 0 || !compare(oldValue, newValue)) {
          atom._snapshot = newValue;
          return true;
        }
        return false;
      } finally {
        activeSub = prevSub;
        if (isComputed) atom.flags &= ~ReactiveFlags.RecursedCheck;
        purgeDeps(atom);
      }
    }
  };
  if (isComputed) {
    atom.flags = ReactiveFlags.Mutable | ReactiveFlags.Dirty;
    atom.get = function() {
      const flags = atom.flags;
      if (flags & ReactiveFlags.Dirty || flags & ReactiveFlags.Pending && checkDirty(atom.deps, atom)) {
        if (atom._update()) {
          const subs = atom.subs;
          if (subs !== void 0) shallowPropagate(subs);
        }
      } else if (flags & ReactiveFlags.Pending) atom.flags = flags & ~ReactiveFlags.Pending;
      if (activeSub !== void 0) link(atom, activeSub, cycle);
      return atom._snapshot;
    };
  } else atom.set = function(valueOrFn2) {
    if (atom._update(valueOrFn2)) {
      const subs = atom.subs;
      if (subs !== void 0) {
        propagate(subs);
        shallowPropagate(subs);
        flush();
      }
    }
  };
  return atom;
}
function effect(fn) {
  const run = () => {
    const prevSub = activeSub;
    activeSub = effectObj;
    ++cycle;
    effectObj.depsTail = void 0;
    effectObj.flags = ReactiveFlags.Watching | ReactiveFlags.RecursedCheck;
    try {
      return fn();
    } finally {
      activeSub = prevSub;
      effectObj.flags &= ~ReactiveFlags.RecursedCheck;
      purgeDeps(effectObj);
    }
  };
  const effectObj = {
    deps: void 0,
    depsTail: void 0,
    subs: void 0,
    subsTail: void 0,
    flags: ReactiveFlags.Watching | ReactiveFlags.RecursedCheck,
    notify() {
      const flags = this.flags;
      if (flags & ReactiveFlags.Dirty || flags & ReactiveFlags.Pending && checkDirty(this.deps, this)) run();
      else this.flags = ReactiveFlags.Watching;
    },
    stop() {
      this.flags = ReactiveFlags.None;
      this.depsTail = void 0;
      purgeDeps(this);
    }
  };
  run();
  return effectObj;
}

// node_modules/@tanstack/lit-table/dist/reactivity.js
function litReactivity() {
  return {
    createOptionsStore: true,
    wrapExternalAtoms: false,
    addSubscription: () => {
      throw new Error("Feature not supported in current reactivity implementation");
    },
    unmount: () => {
      throw new Error("Feature not supported in current reactivity implementation");
    },
    schedule: (fn) => queueMicrotask(() => fn()),
    batch,
    untrack: (fn) => fn(),
    createReadonlyAtom: (fn, options) => {
      return createAtom(() => fn(), { compare: options === null || options === void 0 ? void 0 : options.compare });
    },
    createWritableAtom: (value, options) => {
      return createAtom(value, { compare: options === null || options === void 0 ? void 0 : options.compare });
    }
  };
}

// node_modules/@tanstack/table-core/dist/utils.js
function functionalUpdate(updater, input) {
  return typeof updater === "function" ? updater(input) : updater;
}
function cloneState(value) {
  if (Array.isArray(value)) return value.map(cloneState);
  if (value && typeof value === "object") {
    const proto = Object.getPrototypeOf(value);
    if (proto !== Object.prototype && proto !== null) return value;
    const copy = {};
    const keys = Object.keys(value);
    for (let i6 = 0; i6 < keys.length; i6++) {
      const key = keys[i6];
      copy[key] = cloneState(value[key]);
    }
    return copy;
  }
  return value;
}
function makeStateUpdater(key, instance) {
  return (updater) => {
    var _atoms;
    (((_atoms = instance.options.atoms) === null || _atoms === void 0 ? void 0 : _atoms[key]) ?? instance.baseAtoms[key]).set((old) => functionalUpdate(updater, old));
  };
}
function isFunction(d3) {
  return d3 instanceof Function;
}
function flattenBy(arr, getChildren) {
  const flat = [];
  const recurse = (subArr) => {
    subArr.forEach((item) => {
      flat.push(item);
      const children = getChildren(item);
      if (children.length) recurse(children);
    });
  };
  recurse(arr);
  return flat;
}
var memo = ({ fn, memoDeps, onAfterCompare, onAfterUpdate, onBeforeCompare, onBeforeUpdate }) => {
  let deps = [];
  let result;
  const memoizedFn = (depArgs) => {
    onBeforeCompare === null || onBeforeCompare === void 0 || onBeforeCompare();
    const newDeps = memoDeps === null || memoDeps === void 0 ? void 0 : memoDeps(depArgs);
    let depsChanged = !newDeps || newDeps.length !== (deps === null || deps === void 0 ? void 0 : deps.length);
    if (!depsChanged && newDeps) {
      for (let i6 = 0; i6 < newDeps.length; i6++) if (newDeps[i6] !== deps[i6]) {
        depsChanged = true;
        break;
      }
    }
    onAfterCompare === null || onAfterCompare === void 0 || onAfterCompare(depsChanged);
    if (!depsChanged) return result;
    deps = newDeps;
    onBeforeUpdate === null || onBeforeUpdate === void 0 || onBeforeUpdate();
    result = fn(...newDeps ?? []);
    onAfterUpdate === null || onAfterUpdate === void 0 || onAfterUpdate(result);
    return result;
  };
  return memoizedFn;
};
var pad = (str, num) => {
  str = String(str);
  while (str.length < num) str = " " + str;
  return str;
};
function tableMemo({ feature, fnName, objectId, onAfterUpdate, table, ...memoOptions }) {
  let beforeCompareTime;
  let afterCompareTime;
  let startCalcTime;
  let endCalcTime;
  let runCount = 0;
  let debug;
  let debugCache;
  if (true) {
    const { debugCache: _debugCache, debugAll } = table.options;
    debugCache = _debugCache;
    const { parentName } = getFunctionNameInfo(fnName, ".");
    debug = debugAll || table.options[`debug${(parentName != "table" ? parentName + "s" : parentName).replace(parentName, parentName.charAt(0).toUpperCase() + parentName.slice(1))}`] || (feature ? table.options[`debug${feature.charAt(0).toUpperCase() + feature.slice(1)}`] : false);
  }
  function logTime(time, depsChanged) {
    var _memoOptions$memoDeps;
    const runType = runCount === 0 ? "(1st run)" : depsChanged ? "(rerun #" + runCount + ")" : "(cache)";
    runCount++;
    console.groupCollapsed(`%c\u23F1 ${pad(`${time.toFixed(1)} ms`, 12)} %c${runType}%c ${fnName}%c ${objectId ? `(${fnName.split(".")[0]}Id: ${objectId})` : ""}`, `font-size: .6rem; font-weight: bold; ${depsChanged ? `color: hsl(
        ${Math.max(0, Math.min(120 - Math.log10(time) * 60, 120))}deg 100% 31%);` : ""} `, `color: ${runCount < 2 ? "#FF00FF" : "#FF1493"}`, "color: #666", "color: #87CEEB");
    console.info({
      feature,
      state: table.store.state,
      deps: (_memoOptions$memoDeps = memoOptions.memoDeps) === null || _memoOptions$memoDeps === void 0 ? void 0 : _memoOptions$memoDeps.toString()
    });
    console.trace();
    console.groupEnd();
  }
  const onAfterUpdateHandler = () => {
    if (!onAfterUpdate) return;
    const { schedule, untrack } = table._reactivity;
    schedule(() => untrack(() => onAfterUpdate()));
  };
  const debugOptions = true ? {
    onBeforeCompare: () => {
      if (debugCache) beforeCompareTime = performance.now();
    },
    onAfterCompare: (depsChanged) => {
      if (debugCache) {
        afterCompareTime = performance.now();
        const compareTime = Math.round((afterCompareTime - beforeCompareTime) * 100) / 100;
        if (!depsChanged) logTime(compareTime, depsChanged);
      }
    },
    onBeforeUpdate: () => {
      if (debug) startCalcTime = performance.now();
    },
    onAfterUpdate: () => {
      if (debug) {
        endCalcTime = performance.now();
        logTime(Math.round((endCalcTime - startCalcTime) * 100) / 100, true);
      }
      onAfterUpdateHandler();
    }
  } : { onAfterUpdate: () => {
    onAfterUpdateHandler();
  } };
  return memo({
    ...memoOptions,
    ...debugOptions
  });
}
function getFunctionNameInfo(staticFnName, splitBy = "_") {
  const [parentName, fnKey] = staticFnName.split(splitBy);
  return {
    fnKey,
    fnName: `${parentName}.${fnKey}`,
    parentName
  };
}
function assignTableAPIs(feature, table, apis) {
  for (const [staticFnName, { fn, memoDeps }] of Object.entries(apis)) {
    const { fnKey, fnName } = getFunctionNameInfo(staticFnName);
    table[fnKey] = memoDeps ? tableMemo({
      memoDeps,
      fn,
      fnName,
      table,
      feature
    }) : fn;
  }
}
function assignPrototypeAPIs(feature, prototype, table, apis) {
  for (const [staticFnName, { fn, memoDeps }] of Object.entries(apis)) {
    const { fnKey, fnName } = getFunctionNameInfo(staticFnName);
    if (memoDeps) {
      const memoKey = `_memo_${fnKey}`;
      prototype[fnKey] = function(...args) {
        if (!this[memoKey]) {
          const self = this;
          this[memoKey] = tableMemo({
            memoDeps: (depArgs) => memoDeps(self, depArgs),
            fn: (...deps) => fn(self, ...deps),
            fnName,
            objectId: self.id,
            table,
            feature
          });
        }
        return this[memoKey](...args);
      };
    } else prototype[fnKey] = function(...args) {
      return fn(this, ...args);
    };
  }
}
function callMemoOrStaticFn(obj, fnKey, staticFn, ...args) {
  var _obj$fnKey;
  return ((_obj$fnKey = obj[fnKey]) === null || _obj$fnKey === void 0 ? void 0 : _obj$fnKey.call(obj, ...args)) ?? staticFn(obj, ...args);
}

// node_modules/@tanstack/table-core/dist/core/cells/coreCellsFeature.utils.js
function cell_getValue(cell) {
  return cell.row.getValue(cell.column.id);
}
function cell_renderValue(cell) {
  return cell.getValue() ?? cell.table.options.renderFallbackValue;
}
function cell_getContext(cell) {
  return {
    table: cell.table,
    column: cell.column,
    row: cell.row,
    cell,
    getValue: () => cell.getValue(),
    renderValue: () => cell.renderValue()
  };
}

// node_modules/@tanstack/table-core/dist/core/cells/coreCellsFeature.js
var coreCellsFeature = { assignCellPrototype: (prototype, table) => {
  assignPrototypeAPIs("coreCellsFeature", prototype, table, {
    cell_getValue: { fn: (cell) => cell_getValue(cell) },
    cell_renderValue: { fn: (cell) => cell_renderValue(cell) },
    cell_getContext: {
      fn: (cell) => cell_getContext(cell),
      memoDeps: (cell) => [cell]
    }
  });
} };

// node_modules/@tanstack/table-core/dist/core/headers/constructHeader.js
function getHeaderPrototype(table) {
  if (!table._headerPrototype) {
    table._headerPrototype = { table };
    const features = Object.values(table._features);
    for (let i6 = 0; i6 < features.length; i6++) {
      var _assignHeaderPrototyp, _ref;
      (_assignHeaderPrototyp = (_ref = features[i6]).assignHeaderPrototype) === null || _assignHeaderPrototyp === void 0 || _assignHeaderPrototyp.call(_ref, table._headerPrototype, table);
    }
  }
  return table._headerPrototype;
}
function constructHeader(table, column, options) {
  const headerPrototype = getHeaderPrototype(table);
  const header = Object.create(headerPrototype);
  header.colSpan = 0;
  header.column = column;
  header.depth = options.depth;
  header.headerGroup = null;
  header.id = options.id ?? column.id;
  header.index = options.index;
  header.isPlaceholder = !!options.isPlaceholder;
  header.placeholderId = options.placeholderId;
  header.rowSpan = 0;
  header.subHeaders = [];
  return header;
}

// node_modules/@tanstack/table-core/dist/features/column-pinning/columnPinningFeature.utils.js
function getDefaultColumnPinningState() {
  return {
    left: [],
    right: []
  };
}
function column_pin(column, position) {
  const leafColumns = column.getLeafColumns();
  const columnIds = [];
  for (let i6 = 0; i6 < leafColumns.length; i6++) {
    const id = leafColumns[i6].id;
    if (id) columnIds.push(id);
  }
  table_setColumnPinning(column.table, (old) => {
    if (position === "right") return {
      left: old.left.filter((d3) => !columnIds.includes(d3)),
      right: [...old.right.filter((d3) => !columnIds.includes(d3)), ...columnIds]
    };
    if (position === "left") return {
      left: [...old.left.filter((d3) => !columnIds.includes(d3)), ...columnIds],
      right: old.right.filter((d3) => !columnIds.includes(d3))
    };
    return {
      left: old.left.filter((d3) => !columnIds.includes(d3)),
      right: old.right.filter((d3) => !columnIds.includes(d3))
    };
  });
}
function column_getCanPin(column) {
  return column.getLeafColumns().some((leafColumn) => (leafColumn.columnDef.enablePinning ?? true) && (column.table.options.enableColumnPinning ?? true));
}
function column_getIsPinned(column) {
  var _column$table$atoms$c;
  const leafColumnIds = column.getLeafColumns().map((d3) => d3.id);
  const { left, right } = ((_column$table$atoms$c = column.table.atoms.columnPinning) === null || _column$table$atoms$c === void 0 ? void 0 : _column$table$atoms$c.get()) ?? getDefaultColumnPinningState();
  const isLeft = leafColumnIds.some((d3) => left.includes(d3));
  const isRight = leafColumnIds.some((d3) => right.includes(d3));
  return isLeft ? "left" : isRight ? "right" : false;
}
function column_getPinnedIndex(column) {
  var _column$table$atoms$c2;
  const position = column_getIsPinned(column);
  return position ? ((_column$table$atoms$c2 = column.table.atoms.columnPinning) === null || _column$table$atoms$c2 === void 0 || (_column$table$atoms$c2 = _column$table$atoms$c2.get()) === null || _column$table$atoms$c2 === void 0 ? void 0 : _column$table$atoms$c2[position].indexOf(column.id)) ?? -1 : 0;
}
function row_getCenterVisibleCells(row) {
  var _row$table$atoms$colu;
  const allCells = callMemoOrStaticFn(row, "getVisibleCells", row_getVisibleCells);
  const { left, right } = ((_row$table$atoms$colu = row.table.atoms.columnPinning) === null || _row$table$atoms$colu === void 0 ? void 0 : _row$table$atoms$colu.get()) ?? getDefaultColumnPinningState();
  const leftAndRight = [...left, ...right];
  return allCells.filter((d3) => !leftAndRight.includes(d3.column.id));
}
function row_getLeftVisibleCells(row) {
  var _row$table$atoms$colu2;
  const { left } = ((_row$table$atoms$colu2 = row.table.atoms.columnPinning) === null || _row$table$atoms$colu2 === void 0 ? void 0 : _row$table$atoms$colu2.get()) ?? getDefaultColumnPinningState();
  if (!left.length) return [];
  const allVisibleCells = callMemoOrStaticFn(row, "getVisibleCellsByColumnId", row_getVisibleCellsByColumnId);
  const cells = [];
  for (let i6 = 0; i6 < left.length; i6++) {
    const cell = allVisibleCells[left[i6]];
    if (cell) {
      cell.position = "left";
      cells.push(cell);
    }
  }
  return cells;
}
function row_getRightVisibleCells(row) {
  var _row$table$atoms$colu3;
  const { right } = ((_row$table$atoms$colu3 = row.table.atoms.columnPinning) === null || _row$table$atoms$colu3 === void 0 ? void 0 : _row$table$atoms$colu3.get()) ?? getDefaultColumnPinningState();
  if (!right.length) return [];
  const allVisibleCells = callMemoOrStaticFn(row, "getVisibleCellsByColumnId", row_getVisibleCellsByColumnId);
  const cells = [];
  for (let i6 = 0; i6 < right.length; i6++) {
    const cell = allVisibleCells[right[i6]];
    if (cell) {
      cell.position = "right";
      cells.push(cell);
    }
  }
  return cells;
}
function table_setColumnPinning(table, updater) {
  var _table$options$onColu, _table$options;
  (_table$options$onColu = (_table$options = table.options).onColumnPinningChange) === null || _table$options$onColu === void 0 || _table$options$onColu.call(_table$options, updater);
}
function table_resetColumnPinning(table, defaultState) {
  table_setColumnPinning(table, defaultState ? getDefaultColumnPinningState() : cloneState(table.initialState.columnPinning ?? getDefaultColumnPinningState()));
}
function table_getIsSomeColumnsPinned(table, position) {
  var _table$atoms$columnPi;
  const pinningState = (_table$atoms$columnPi = table.atoms.columnPinning) === null || _table$atoms$columnPi === void 0 ? void 0 : _table$atoms$columnPi.get();
  if (!position) return Boolean((pinningState === null || pinningState === void 0 ? void 0 : pinningState.left.length) || (pinningState === null || pinningState === void 0 ? void 0 : pinningState.right.length));
  return Boolean(pinningState === null || pinningState === void 0 ? void 0 : pinningState[position].length);
}
function table_getLeftHeaderGroups(table) {
  var _table$atoms$columnPi2;
  const allColumns = table.getAllColumns();
  const leafColumnsById = table.getAllLeafColumnsById();
  const { left } = ((_table$atoms$columnPi2 = table.atoms.columnPinning) === null || _table$atoms$columnPi2 === void 0 ? void 0 : _table$atoms$columnPi2.get()) ?? getDefaultColumnPinningState();
  const orderedLeafColumns = [];
  for (let i6 = 0; i6 < left.length; i6++) {
    const column = leafColumnsById[left[i6]];
    if (column && callMemoOrStaticFn(column, "getIsVisible", column_getIsVisible)) orderedLeafColumns.push(column);
  }
  return buildHeaderGroups(allColumns, orderedLeafColumns, table, "left");
}
function table_getRightHeaderGroups(table) {
  var _table$atoms$columnPi3;
  const allColumns = table.getAllColumns();
  const leafColumnsById = table.getAllLeafColumnsById();
  const { right } = ((_table$atoms$columnPi3 = table.atoms.columnPinning) === null || _table$atoms$columnPi3 === void 0 ? void 0 : _table$atoms$columnPi3.get()) ?? getDefaultColumnPinningState();
  const orderedLeafColumns = [];
  for (let i6 = 0; i6 < right.length; i6++) {
    const column = leafColumnsById[right[i6]];
    if (column && callMemoOrStaticFn(column, "getIsVisible", column_getIsVisible)) orderedLeafColumns.push(column);
  }
  return buildHeaderGroups(allColumns, orderedLeafColumns, table, "right");
}
function table_getCenterHeaderGroups(table) {
  var _table$atoms$columnPi4;
  const allColumns = table.getAllColumns();
  let leafColumns = callMemoOrStaticFn(table, "getVisibleLeafColumns", table_getVisibleLeafColumns);
  const { left, right } = ((_table$atoms$columnPi4 = table.atoms.columnPinning) === null || _table$atoms$columnPi4 === void 0 ? void 0 : _table$atoms$columnPi4.get()) ?? getDefaultColumnPinningState();
  const leftAndRight = [...left, ...right];
  leafColumns = leafColumns.filter((column) => !leftAndRight.includes(column.id));
  return buildHeaderGroups(allColumns, leafColumns, table, "center");
}
function table_getLeftFooterGroups(table) {
  return [...callMemoOrStaticFn(table, "getLeftHeaderGroups", table_getLeftHeaderGroups)].reverse();
}
function table_getRightFooterGroups(table) {
  return [...callMemoOrStaticFn(table, "getRightHeaderGroups", table_getRightHeaderGroups)].reverse();
}
function table_getCenterFooterGroups(table) {
  return [...callMemoOrStaticFn(table, "getCenterHeaderGroups", table_getCenterHeaderGroups)].reverse();
}
function table_getLeftFlatHeaders(table) {
  const leftHeaderGroups = callMemoOrStaticFn(table, "getLeftHeaderGroups", table_getLeftHeaderGroups);
  const result = [];
  for (let i6 = 0; i6 < leftHeaderGroups.length; i6++) {
    const headers = leftHeaderGroups[i6].headers;
    for (let j2 = 0; j2 < headers.length; j2++) result.push(headers[j2]);
  }
  return result;
}
function table_getRightFlatHeaders(table) {
  const rightHeaderGroups = callMemoOrStaticFn(table, "getRightHeaderGroups", table_getRightHeaderGroups);
  const result = [];
  for (let i6 = 0; i6 < rightHeaderGroups.length; i6++) {
    const headers = rightHeaderGroups[i6].headers;
    for (let j2 = 0; j2 < headers.length; j2++) result.push(headers[j2]);
  }
  return result;
}
function table_getCenterFlatHeaders(table) {
  const centerHeaderGroups = callMemoOrStaticFn(table, "getCenterHeaderGroups", table_getCenterHeaderGroups);
  const result = [];
  for (let i6 = 0; i6 < centerHeaderGroups.length; i6++) {
    const headers = centerHeaderGroups[i6].headers;
    for (let j2 = 0; j2 < headers.length; j2++) result.push(headers[j2]);
  }
  return result;
}
function table_getLeftLeafHeaders(table) {
  return callMemoOrStaticFn(table, "getLeftFlatHeaders", table_getLeftFlatHeaders).filter((header) => !header.subHeaders.length);
}
function table_getRightLeafHeaders(table) {
  return callMemoOrStaticFn(table, "getRightFlatHeaders", table_getRightFlatHeaders).filter((header) => !header.subHeaders.length);
}
function table_getCenterLeafHeaders(table) {
  return callMemoOrStaticFn(table, "getCenterFlatHeaders", table_getCenterFlatHeaders).filter((header) => !header.subHeaders.length);
}
function table_getLeftLeafColumns(table) {
  var _table$atoms$columnPi5;
  const { left } = ((_table$atoms$columnPi5 = table.atoms.columnPinning) === null || _table$atoms$columnPi5 === void 0 ? void 0 : _table$atoms$columnPi5.get()) ?? getDefaultColumnPinningState();
  const leafColumnsById = table.getAllLeafColumnsById();
  const result = [];
  for (let i6 = 0; i6 < left.length; i6++) {
    const column = leafColumnsById[left[i6]];
    if (column) result.push(column);
  }
  return result;
}
function table_getRightLeafColumns(table) {
  var _table$atoms$columnPi6;
  const { right } = ((_table$atoms$columnPi6 = table.atoms.columnPinning) === null || _table$atoms$columnPi6 === void 0 ? void 0 : _table$atoms$columnPi6.get()) ?? getDefaultColumnPinningState();
  const leafColumnsById = table.getAllLeafColumnsById();
  const result = [];
  for (let i6 = 0; i6 < right.length; i6++) {
    const column = leafColumnsById[right[i6]];
    if (column) result.push(column);
  }
  return result;
}
function table_getCenterLeafColumns(table) {
  var _table$atoms$columnPi7;
  const { left, right } = ((_table$atoms$columnPi7 = table.atoms.columnPinning) === null || _table$atoms$columnPi7 === void 0 ? void 0 : _table$atoms$columnPi7.get()) ?? getDefaultColumnPinningState();
  const leftAndRight = [...left, ...right];
  return table.getAllLeafColumns().filter((d3) => !leftAndRight.includes(d3.id));
}
function table_getPinnedLeafColumns(table, position) {
  return !position ? table.getAllLeafColumns() : position === "left" ? callMemoOrStaticFn(table, "getLeftLeafColumns", table_getLeftLeafColumns) : position === "right" ? callMemoOrStaticFn(table, "getRightLeafColumns", table_getRightLeafColumns) : callMemoOrStaticFn(table, "getCenterLeafColumns", table_getCenterLeafColumns);
}
function table_getLeftVisibleLeafColumns(table) {
  return callMemoOrStaticFn(table, "getLeftLeafColumns", table_getLeftLeafColumns).filter((column) => callMemoOrStaticFn(column, "getIsVisible", column_getIsVisible));
}
function table_getRightVisibleLeafColumns(table) {
  return callMemoOrStaticFn(table, "getRightLeafColumns", table_getRightLeafColumns).filter((column) => callMemoOrStaticFn(column, "getIsVisible", column_getIsVisible));
}
function table_getCenterVisibleLeafColumns(table) {
  return callMemoOrStaticFn(table, "getCenterLeafColumns", table_getCenterLeafColumns).filter((column) => callMemoOrStaticFn(column, "getIsVisible", column_getIsVisible));
}
function table_getPinnedVisibleLeafColumns(table, position) {
  return !position ? callMemoOrStaticFn(table, "getVisibleLeafColumns", table_getVisibleLeafColumns) : position === "left" ? callMemoOrStaticFn(table, "getLeftVisibleLeafColumns", table_getLeftVisibleLeafColumns) : position === "right" ? callMemoOrStaticFn(table, "getRightVisibleLeafColumns", table_getRightVisibleLeafColumns) : callMemoOrStaticFn(table, "getCenterVisibleLeafColumns", table_getCenterVisibleLeafColumns);
}

// node_modules/@tanstack/table-core/dist/features/column-visibility/columnVisibilityFeature.utils.js
function getDefaultColumnVisibilityState() {
  return {};
}
function column_toggleVisibility(column, visible) {
  if (column_getCanHide(column)) table_setColumnVisibility(column.table, (old) => ({
    ...old,
    [column.id]: visible ?? !callMemoOrStaticFn(column, "getIsVisible", column_getIsVisible)
  }));
}
function column_getIsVisible(column) {
  var _column$table$atoms$c;
  const childColumns = column.columns;
  return (childColumns.length ? childColumns.some((childColumn) => callMemoOrStaticFn(childColumn, "getIsVisible", column_getIsVisible)) : (_column$table$atoms$c = column.table.atoms.columnVisibility) === null || _column$table$atoms$c === void 0 || (_column$table$atoms$c = _column$table$atoms$c.get()) === null || _column$table$atoms$c === void 0 ? void 0 : _column$table$atoms$c[column.id]) ?? true;
}
function column_getCanHide(column) {
  return (column.columnDef.enableHiding ?? true) && (column.table.options.enableHiding ?? true);
}
function column_getToggleVisibilityHandler(column) {
  return (e6) => {
    column_toggleVisibility(column, e6.target.checked);
  };
}
function row_getVisibleCells(row) {
  var _row$table$atoms$colu;
  const allCells = row.getAllCells();
  const visibleCells = [];
  for (let i6 = 0; i6 < allCells.length; i6++) {
    const cell = allCells[i6];
    if (callMemoOrStaticFn(cell.column, "getIsVisible", column_getIsVisible)) visibleCells.push(cell);
  }
  const { left, right } = ((_row$table$atoms$colu = row.table.atoms.columnPinning) === null || _row$table$atoms$colu === void 0 ? void 0 : _row$table$atoms$colu.get()) ?? getDefaultColumnPinningState();
  if (!left.length && !right.length) return visibleCells;
  const visibleCellsByColumnId = callMemoOrStaticFn(row, "getVisibleCellsByColumnId", row_getVisibleCellsByColumnId);
  const leftCells = [];
  for (let i6 = 0; i6 < left.length; i6++) {
    const cell = visibleCellsByColumnId[left[i6]];
    if (cell) leftCells.push(cell);
  }
  const rightCells = [];
  for (let i6 = 0; i6 < right.length; i6++) {
    const cell = visibleCellsByColumnId[right[i6]];
    if (cell) rightCells.push(cell);
  }
  const centerCells = [];
  for (let i6 = 0; i6 < visibleCells.length; i6++) {
    const cell = visibleCells[i6];
    const id = cell.column.id;
    if (!left.includes(id) && !right.includes(id)) centerCells.push(cell);
  }
  return [
    ...leftCells,
    ...centerCells,
    ...rightCells
  ];
}
function row_getVisibleCellsByColumnId(row) {
  const result = {};
  const allCells = row.getAllCells();
  for (let i6 = 0; i6 < allCells.length; i6++) {
    const cell = allCells[i6];
    if (callMemoOrStaticFn(cell.column, "getIsVisible", column_getIsVisible)) result[cell.column.id] = cell;
  }
  return result;
}
function table_getVisibleFlatColumns(table) {
  return table.getAllFlatColumns().filter((column) => callMemoOrStaticFn(column, "getIsVisible", column_getIsVisible));
}
function table_getVisibleLeafColumns(table) {
  return table.getAllLeafColumns().filter((column) => callMemoOrStaticFn(column, "getIsVisible", column_getIsVisible));
}
function table_setColumnVisibility(table, updater) {
  var _table$options$onColu, _table$options;
  (_table$options$onColu = (_table$options = table.options).onColumnVisibilityChange) === null || _table$options$onColu === void 0 || _table$options$onColu.call(_table$options, updater);
}
function table_resetColumnVisibility(table, defaultState) {
  table_setColumnVisibility(table, defaultState ? {} : cloneState(table.initialState.columnVisibility ?? {}));
}
function table_toggleAllColumnsVisible(table, value) {
  value = value ?? !table_getIsAllColumnsVisible(table);
  const visibility = {};
  const leafColumns = table.getAllLeafColumns();
  for (let i6 = 0; i6 < leafColumns.length; i6++) {
    const column = leafColumns[i6];
    visibility[column.id] = !value ? !column_getCanHide(column) : value;
  }
  table_setColumnVisibility(table, visibility);
}
function table_getIsAllColumnsVisible(table) {
  return !table.getAllLeafColumns().some((column) => !callMemoOrStaticFn(column, "getIsVisible", column_getIsVisible));
}
function table_getIsSomeColumnsVisible(table) {
  return table.getAllLeafColumns().some((column) => callMemoOrStaticFn(column, "getIsVisible", column_getIsVisible));
}
function table_getToggleAllColumnsVisibilityHandler(table) {
  return (e6) => {
    table_toggleAllColumnsVisible(table, e6.target.checked);
  };
}

// node_modules/@tanstack/table-core/dist/core/headers/buildHeaderGroups.js
function buildHeaderGroups(allColumns, columnsToGroup, table, headerFamily) {
  var _headerGroups$;
  let maxDepth = 0;
  const findMaxDepth = (columns, depth = 1) => {
    maxDepth = Math.max(maxDepth, depth);
    for (let i6 = 0; i6 < columns.length; i6++) {
      const column = columns[i6];
      if (callMemoOrStaticFn(column, "getIsVisible", column_getIsVisible)) {
        if (column.columns.length) findMaxDepth(column.columns, depth + 1);
      }
    }
  };
  findMaxDepth(allColumns);
  const headerGroups = [];
  const constructHeaderGroup = (headersToGroup, depth) => {
    const headerGroup = {
      depth,
      id: [headerFamily, `${depth}`].filter(Boolean).join("_"),
      headers: []
    };
    const pendingParentHeaders = [];
    headersToGroup.forEach((headerToGroup) => {
      const latestPendingParentHeader = pendingParentHeaders[pendingParentHeaders.length - 1];
      const isLeafHeader = headerToGroup.column.depth === headerGroup.depth;
      let column;
      let isPlaceholder = false;
      if (isLeafHeader && headerToGroup.column.parent) column = headerToGroup.column.parent;
      else {
        column = headerToGroup.column;
        isPlaceholder = true;
      }
      if (latestPendingParentHeader && latestPendingParentHeader.column === column) latestPendingParentHeader.subHeaders.push(headerToGroup);
      else {
        const header = constructHeader(table, column, {
          id: [
            headerFamily,
            depth,
            column.id,
            headerToGroup.id
          ].filter(Boolean).join("_"),
          isPlaceholder,
          placeholderId: isPlaceholder ? `${pendingParentHeaders.filter((d3) => d3.column === column).length}` : void 0,
          depth,
          index: pendingParentHeaders.length
        });
        header.subHeaders.push(headerToGroup);
        pendingParentHeaders.push(header);
      }
      headerGroup.headers.push(headerToGroup);
      headerToGroup.headerGroup = headerGroup;
    });
    headerGroups.push(headerGroup);
    if (depth > 0) constructHeaderGroup(pendingParentHeaders, depth - 1);
  };
  constructHeaderGroup(columnsToGroup.map((column, index) => constructHeader(table, column, {
    depth: maxDepth,
    index
  })), maxDepth - 1);
  headerGroups.reverse();
  const recurseHeadersForSpans = (headers) => {
    const results = [];
    for (let i6 = 0; i6 < headers.length; i6++) {
      const header = headers[i6];
      if (!callMemoOrStaticFn(header.column, "getIsVisible", column_getIsVisible)) continue;
      let colSpan = 0;
      let minChildRowSpan = Infinity;
      if (header.subHeaders.length) {
        const childSpans = recurseHeadersForSpans(header.subHeaders);
        for (let j2 = 0; j2 < childSpans.length; j2++) {
          const child = childSpans[j2];
          colSpan += child.colSpan;
          if (child.rowSpan < minChildRowSpan) minChildRowSpan = child.rowSpan;
        }
      } else {
        colSpan = 1;
        minChildRowSpan = 0;
      }
      header.colSpan = colSpan;
      header.rowSpan = minChildRowSpan;
      results.push({
        colSpan,
        rowSpan: header.rowSpan
      });
    }
    return results;
  };
  recurseHeadersForSpans(((_headerGroups$ = headerGroups[0]) === null || _headerGroups$ === void 0 ? void 0 : _headerGroups$.headers) ?? []);
  return headerGroups;
}

// node_modules/@tanstack/table-core/dist/core/columns/constructColumn.js
function getColumnPrototype(table) {
  if (!table._columnPrototype) {
    table._columnPrototype = { table };
    const features = Object.values(table._features);
    for (let i6 = 0; i6 < features.length; i6++) {
      var _assignColumnPrototyp, _ref;
      (_assignColumnPrototyp = (_ref = features[i6]).assignColumnPrototype) === null || _assignColumnPrototyp === void 0 || _assignColumnPrototyp.call(_ref, table._columnPrototype, table);
    }
  }
  return table._columnPrototype;
}
function constructColumn(table, columnDef, depth, parent) {
  const resolvedColumnDef = {
    ...table.getDefaultColumnDef(),
    ...columnDef
  };
  const accessorKey = resolvedColumnDef.accessorKey;
  const id = resolvedColumnDef.id ?? (accessorKey ? accessorKey.replaceAll(".", "_") : void 0) ?? (typeof resolvedColumnDef.header === "string" ? resolvedColumnDef.header : void 0);
  let accessorFn;
  if (resolvedColumnDef.accessorFn) accessorFn = resolvedColumnDef.accessorFn;
  else if (accessorKey) if (accessorKey.includes(".")) {
    const keys = accessorKey.split(".");
    accessorFn = (originalRow) => {
      let result = originalRow;
      for (let i6 = 0; i6 < keys.length; i6++) {
        const key = keys[i6];
        result = result === null || result === void 0 ? void 0 : result[key];
        if (result === void 0) console.warn(`"${key}" in deeply nested key "${accessorKey}" returned undefined.`);
      }
      return result;
    };
  } else accessorFn = (originalRow) => originalRow[resolvedColumnDef.accessorKey];
  if (!id) {
    if (true) throw new Error(resolvedColumnDef.accessorFn ? `coreColumnsFeature require an id when using an accessorFn` : `coreColumnsFeature require an id when using a non-string header`);
    throw new Error();
  }
  const columnPrototype = getColumnPrototype(table);
  const column = Object.create(columnPrototype);
  column.accessorFn = accessorFn;
  column.columnDef = resolvedColumnDef;
  column.columns = [];
  column.depth = depth;
  column.id = `${String(id)}`;
  column.parent = parent;
  return column;
}

// node_modules/@tanstack/table-core/dist/features/column-ordering/columnOrderingFeature.utils.js
function column_getIndex(column, position) {
  return table_getPinnedVisibleLeafColumns(column.table, position).findIndex((d3) => d3.id === column.id);
}
function table_getOrderColumnsFn(table) {
  var _table$atoms$columnOr;
  const columnOrder = (_table$atoms$columnOr = table.atoms.columnOrder) === null || _table$atoms$columnOr === void 0 ? void 0 : _table$atoms$columnOr.get();
  return (columns) => {
    let orderedColumns = [];
    if (!(columnOrder === null || columnOrder === void 0 ? void 0 : columnOrder.length)) orderedColumns = columns;
    else {
      const remaining = /* @__PURE__ */ new Map();
      for (let i6 = 0; i6 < columns.length; i6++) {
        const column = columns[i6];
        remaining.set(column.id, column);
      }
      for (let i6 = 0; i6 < columnOrder.length; i6++) {
        const id = columnOrder[i6];
        const column = remaining.get(id);
        if (column) {
          orderedColumns.push(column);
          remaining.delete(id);
        }
      }
      for (let i6 = 0; i6 < columns.length; i6++) {
        const column = columns[i6];
        if (remaining.has(column.id)) orderedColumns.push(column);
      }
    }
    return orderColumns(table, orderedColumns);
  };
}
function orderColumns(table, leafColumns) {
  var _table$atoms$grouping;
  const grouping = ((_table$atoms$grouping = table.atoms.grouping) === null || _table$atoms$grouping === void 0 ? void 0 : _table$atoms$grouping.get()) ?? [];
  const { groupedColumnMode } = table.options;
  if (!grouping.length || !groupedColumnMode) return leafColumns;
  const nonGroupingColumns = leafColumns.filter((col) => !grouping.includes(col.id));
  if (groupedColumnMode === "remove") return nonGroupingColumns;
  const leafColumnsById = /* @__PURE__ */ new Map();
  for (let i6 = 0; i6 < leafColumns.length; i6++) {
    const col = leafColumns[i6];
    leafColumnsById.set(col.id, col);
  }
  const groupingColumns = [];
  for (let i6 = 0; i6 < grouping.length; i6++) {
    const col = leafColumnsById.get(grouping[i6]);
    if (col) groupingColumns.push(col);
  }
  return [...groupingColumns, ...nonGroupingColumns];
}

// node_modules/@tanstack/table-core/dist/core/columns/coreColumnsFeature.utils.js
function column_getFlatColumns(column) {
  return [column, ...column.columns.flatMap((col) => col.getFlatColumns())];
}
function column_getLeafColumns(column) {
  if (column.columns.length) {
    const leafColumns = column.columns.flatMap((col) => col.getLeafColumns());
    return callMemoOrStaticFn(column.table, "getOrderColumns", table_getOrderColumnsFn)(leafColumns);
  }
  return [column];
}
function table_getDefaultColumnDef(table) {
  return {
    header: (props) => {
      const resolvedColumnDef = props.header.column.columnDef;
      if (resolvedColumnDef.accessorKey) return resolvedColumnDef.accessorKey;
      if (resolvedColumnDef.accessorFn) return resolvedColumnDef.id;
      return null;
    },
    cell: (props) => {
      var _props$renderValue, _props$renderValue$to;
      return ((_props$renderValue = props.renderValue()) === null || _props$renderValue === void 0 || (_props$renderValue$to = _props$renderValue.toString) === null || _props$renderValue$to === void 0 ? void 0 : _props$renderValue$to.call(_props$renderValue)) ?? null;
    },
    ...Object.values(table._features).reduce((obj, feature) => {
      var _feature$getDefaultCo;
      return Object.assign(obj, (_feature$getDefaultCo = feature.getDefaultColumnDef) === null || _feature$getDefaultCo === void 0 ? void 0 : _feature$getDefaultCo.call(feature));
    }, {}),
    ...table.options.defaultColumn
  };
}
function table_getAllColumns(table) {
  const recurseColumns = (colDefs, parent, depth = 0) => {
    return colDefs.map((columnDef) => {
      const column = constructColumn(table, columnDef, depth, parent);
      const groupingColumnDef = columnDef;
      column.columns = groupingColumnDef.columns ? recurseColumns(groupingColumnDef.columns, column, depth + 1) : [];
      return column;
    });
  };
  return recurseColumns(table.options.columns);
}
function table_getAllFlatColumns(table) {
  return table.getAllColumns().flatMap((column) => column.getFlatColumns());
}
function table_getAllFlatColumnsById(table) {
  const result = {};
  const flatColumns = table.getAllFlatColumns();
  for (let i6 = 0; i6 < flatColumns.length; i6++) {
    const column = flatColumns[i6];
    result[column.id] = column;
  }
  return result;
}
function table_getAllLeafColumns(table) {
  const leafColumns = table.getAllColumns().flatMap((c5) => c5.getLeafColumns());
  return callMemoOrStaticFn(table, "getOrderColumns", table_getOrderColumnsFn)(leafColumns);
}
function table_getAllLeafColumnsById(table) {
  const result = {};
  const leafColumns = table.getAllLeafColumns();
  for (let i6 = 0; i6 < leafColumns.length; i6++) {
    const column = leafColumns[i6];
    result[column.id] = column;
  }
  return result;
}
function table_getColumn(table, columnId) {
  const column = table.getAllFlatColumnsById()[columnId];
  if (!column) console.warn(`[Table] Column with id '${columnId}' does not exist.`);
  return column;
}

// node_modules/@tanstack/table-core/dist/core/columns/coreColumnsFeature.js
var coreColumnsFeature = {
  assignColumnPrototype: (prototype, table) => {
    assignPrototypeAPIs("coreColumnsFeature", prototype, table, {
      column_getFlatColumns: {
        fn: (column) => column_getFlatColumns(column),
        memoDeps: (column) => [column.table.options.columns]
      },
      column_getLeafColumns: {
        fn: (column) => column_getLeafColumns(column),
        memoDeps: (column) => {
          var _column$table$atoms$c, _column$table$atoms$g;
          return [
            (_column$table$atoms$c = column.table.atoms.columnOrder) === null || _column$table$atoms$c === void 0 ? void 0 : _column$table$atoms$c.get(),
            (_column$table$atoms$g = column.table.atoms.grouping) === null || _column$table$atoms$g === void 0 ? void 0 : _column$table$atoms$g.get(),
            column.table.options.columns,
            column.table.options.groupedColumnMode
          ];
        }
      }
    });
  },
  constructTableAPIs: (table) => {
    assignTableAPIs("coreColumnsFeature", table, {
      table_getDefaultColumnDef: {
        fn: () => table_getDefaultColumnDef(table),
        memoDeps: () => [table.options.defaultColumn]
      },
      table_getAllColumns: {
        fn: () => table_getAllColumns(table),
        memoDeps: () => [table.options.columns]
      },
      table_getAllFlatColumns: {
        fn: () => table_getAllFlatColumns(table),
        memoDeps: () => [table.options.columns]
      },
      table_getAllFlatColumnsById: {
        fn: () => table_getAllFlatColumnsById(table),
        memoDeps: () => [table.options.columns]
      },
      table_getAllLeafColumns: {
        fn: () => table_getAllLeafColumns(table),
        memoDeps: () => {
          var _table$atoms$columnOr, _table$atoms$grouping;
          return [
            (_table$atoms$columnOr = table.atoms.columnOrder) === null || _table$atoms$columnOr === void 0 ? void 0 : _table$atoms$columnOr.get(),
            (_table$atoms$grouping = table.atoms.grouping) === null || _table$atoms$grouping === void 0 ? void 0 : _table$atoms$grouping.get(),
            table.options.columns,
            table.options.groupedColumnMode
          ];
        }
      },
      table_getAllLeafColumnsById: {
        fn: () => table_getAllLeafColumnsById(table),
        memoDeps: () => [table.getAllLeafColumns()]
      },
      table_getColumn: { fn: (columnId) => table_getColumn(table, columnId) }
    });
  }
};

// node_modules/@tanstack/table-core/dist/core/headers/coreHeadersFeature.utils.js
function header_getLeafHeaders(header) {
  const leafHeaders = [];
  const recurseHeader = (h5) => {
    if (h5.subHeaders.length) h5.subHeaders.map(recurseHeader);
    leafHeaders.push(h5);
  };
  recurseHeader(header);
  return leafHeaders;
}
function header_getContext(header) {
  return {
    column: header.column,
    header,
    table: header.column.table
  };
}
function table_getHeaderGroups(table) {
  var _table$atoms$columnPi;
  const { left, right } = ((_table$atoms$columnPi = table.atoms.columnPinning) === null || _table$atoms$columnPi === void 0 ? void 0 : _table$atoms$columnPi.get()) ?? getDefaultColumnPinningState();
  const allColumns = table.getAllColumns();
  const leafColumns = callMemoOrStaticFn(table, "getVisibleLeafColumns", table_getVisibleLeafColumns);
  if (!left.length && !right.length) return buildHeaderGroups(allColumns, leafColumns, table);
  const leafColumnsById = table.getAllLeafColumnsById();
  const leftColumns = [];
  for (let i6 = 0; i6 < left.length; i6++) {
    const column = leafColumnsById[left[i6]];
    if (column && callMemoOrStaticFn(column, "getIsVisible", column_getIsVisible)) leftColumns.push(column);
  }
  const rightColumns = [];
  for (let i6 = 0; i6 < right.length; i6++) {
    const column = leafColumnsById[right[i6]];
    if (column && callMemoOrStaticFn(column, "getIsVisible", column_getIsVisible)) rightColumns.push(column);
  }
  const centerColumns = leafColumns.filter((column) => !left.includes(column.id) && !right.includes(column.id));
  return buildHeaderGroups(allColumns, [
    ...leftColumns,
    ...centerColumns,
    ...rightColumns
  ], table);
}
function table_getFooterGroups(table) {
  return [...table.getHeaderGroups()].reverse();
}
function table_getFlatHeaders(table) {
  const headerGroups = table.getHeaderGroups();
  const result = [];
  for (let i6 = 0; i6 < headerGroups.length; i6++) {
    const headers = headerGroups[i6].headers;
    for (let j2 = 0; j2 < headers.length; j2++) result.push(headers[j2]);
  }
  return result;
}
function table_getLeafHeaders(table) {
  var _table$getHeaderGroup;
  const topHeaders = ((_table$getHeaderGroup = table.getHeaderGroups()[0]) === null || _table$getHeaderGroup === void 0 ? void 0 : _table$getHeaderGroup.headers) ?? [];
  const result = [];
  for (let i6 = 0; i6 < topHeaders.length; i6++) {
    const leafHeaders = topHeaders[i6].getLeafHeaders();
    for (let j2 = 0; j2 < leafHeaders.length; j2++) result.push(leafHeaders[j2]);
  }
  return result;
}

// node_modules/@tanstack/table-core/dist/core/headers/coreHeadersFeature.js
var coreHeadersFeature = {
  assignHeaderPrototype: (prototype, table) => {
    assignPrototypeAPIs("coreHeadersFeature", prototype, table, {
      header_getLeafHeaders: {
        fn: (header) => header_getLeafHeaders(header),
        memoDeps: (header) => [header.column.table.options.columns]
      },
      header_getContext: {
        fn: (header) => header_getContext(header),
        memoDeps: (header) => [header.column.table.options.columns]
      }
    });
  },
  constructTableAPIs: (table) => {
    assignTableAPIs("coreHeadersFeature", table, {
      table_getHeaderGroups: {
        fn: () => table_getHeaderGroups(table),
        memoDeps: () => {
          var _table$atoms$columnOr, _table$atoms$grouping, _table$atoms$columnPi, _table$atoms$columnVi;
          return [
            table.options.columns,
            (_table$atoms$columnOr = table.atoms.columnOrder) === null || _table$atoms$columnOr === void 0 ? void 0 : _table$atoms$columnOr.get(),
            (_table$atoms$grouping = table.atoms.grouping) === null || _table$atoms$grouping === void 0 ? void 0 : _table$atoms$grouping.get(),
            (_table$atoms$columnPi = table.atoms.columnPinning) === null || _table$atoms$columnPi === void 0 ? void 0 : _table$atoms$columnPi.get(),
            (_table$atoms$columnVi = table.atoms.columnVisibility) === null || _table$atoms$columnVi === void 0 ? void 0 : _table$atoms$columnVi.get(),
            table.options.groupedColumnMode
          ];
        }
      },
      table_getFooterGroups: {
        fn: () => table_getFooterGroups(table),
        memoDeps: () => [table.getHeaderGroups()]
      },
      table_getFlatHeaders: {
        fn: () => table_getFlatHeaders(table),
        memoDeps: () => [table.getHeaderGroups()]
      },
      table_getLeafHeaders: {
        fn: () => table_getLeafHeaders(table),
        memoDeps: () => [table.getHeaderGroups()]
      }
    });
  }
};

// node_modules/@tanstack/table-core/dist/core/rows/constructRow.js
function getRowPrototype(table) {
  if (!table._rowPrototype) {
    table._rowPrototype = { table };
    const features = Object.values(table._features);
    for (let i6 = 0; i6 < features.length; i6++) {
      var _assignRowPrototype, _ref;
      (_assignRowPrototype = (_ref = features[i6]).assignRowPrototype) === null || _assignRowPrototype === void 0 || _assignRowPrototype.call(_ref, table._rowPrototype, table);
    }
  }
  return table._rowPrototype;
}
var constructRow = (table, id, original, rowIndex, depth, subRows, parentId) => {
  const rowPrototype = getRowPrototype(table);
  const row = Object.create(rowPrototype);
  row._uniqueValuesCache = {};
  row._valuesCache = {};
  row.depth = depth;
  row.id = id;
  row.index = rowIndex;
  row.original = original;
  row.parentId = parentId;
  row.subRows = subRows ?? [];
  const features = Object.values(table._features);
  for (let i6 = 0; i6 < features.length; i6++) {
    var _initRowInstanceData, _ref2;
    (_initRowInstanceData = (_ref2 = features[i6]).initRowInstanceData) === null || _initRowInstanceData === void 0 || _initRowInstanceData.call(_ref2, row);
  }
  return row;
};

// node_modules/@tanstack/table-core/dist/features/row-pagination/rowPaginationFeature.utils.js
var defaultPageIndex = 0;
function table_autoResetPageIndex(table) {
  if (table.options.autoResetAll ?? table.options.autoResetPageIndex ?? !table.options.manualPagination) table_resetPageIndex(table);
}
function table_setPagination(table, updater) {
  var _table$options$onPagi, _table$options;
  const safeUpdater = (old) => {
    return functionalUpdate(updater, old);
  };
  return (_table$options$onPagi = (_table$options = table.options).onPaginationChange) === null || _table$options$onPagi === void 0 ? void 0 : _table$options$onPagi.call(_table$options, safeUpdater);
}
function table_setPageIndex(table, updater) {
  table_setPagination(table, (old) => {
    let pageIndex = functionalUpdate(updater, old.pageIndex);
    const maxPageIndex = typeof table.options.pageCount === "undefined" || table.options.pageCount === -1 ? Number.MAX_SAFE_INTEGER : table.options.pageCount - 1;
    pageIndex = Math.max(0, Math.min(pageIndex, maxPageIndex));
    return {
      ...old,
      pageIndex
    };
  });
}
function table_resetPageIndex(table, defaultState) {
  var _table$atoms$paginati, _table$initialState$p;
  const currentPageIndex = ((_table$atoms$paginati = table.atoms.pagination) === null || _table$atoms$paginati === void 0 || (_table$atoms$paginati = _table$atoms$paginati.get()) === null || _table$atoms$paginati === void 0 ? void 0 : _table$atoms$paginati.pageIndex) ?? defaultPageIndex;
  const newPageIndex = defaultState ? defaultPageIndex : ((_table$initialState$p = table.initialState.pagination) === null || _table$initialState$p === void 0 ? void 0 : _table$initialState$p.pageIndex) ?? defaultPageIndex;
  if (newPageIndex === currentPageIndex) return;
  table_setPageIndex(table, newPageIndex);
}

// node_modules/@tanstack/table-core/dist/core/row-models/createCoreRowModel.js
function createCoreRowModel() {
  return (table) => {
    return tableMemo({
      feature: "coreRowModelsFeature",
      table,
      fnName: "table.getCoreRowModel",
      memoDeps: () => [table.options.data],
      fn: () => _createCoreRowModel(table, table.options.data),
      onAfterUpdate: () => table_autoResetPageIndex(table)
    });
  };
}
function _createCoreRowModel(table, data) {
  const rowModel = {
    rows: [],
    flatRows: [],
    rowsById: {}
  };
  const accessRows = (originalRows, depth = 0, parentRow) => {
    const rows = [];
    for (let i6 = 0; i6 < originalRows.length; i6++) {
      const originalRow = originalRows[i6];
      const row = constructRow(table, table.getRowId(originalRow, i6, parentRow), originalRow, i6, depth, void 0, parentRow === null || parentRow === void 0 ? void 0 : parentRow.id);
      rowModel.flatRows.push(row);
      rowModel.rowsById[row.id] = row;
      rows.push(row);
      if (table.options.getSubRows) {
        var _row$originalSubRows;
        row.originalSubRows = table.options.getSubRows(originalRow, i6);
        if ((_row$originalSubRows = row.originalSubRows) === null || _row$originalSubRows === void 0 ? void 0 : _row$originalSubRows.length) row.subRows = accessRows(row.originalSubRows, depth + 1, row);
      }
    }
    return rows;
  };
  rowModel.rows = accessRows(data);
  return rowModel;
}

// node_modules/@tanstack/table-core/dist/core/row-models/coreRowModelsFeature.utils.js
function table_getCoreRowModel(table) {
  if (!table._rowModels.coreRowModel) {
    var _table$options$featur, _table$options$featur2;
    table._rowModels.coreRowModel = ((_table$options$featur = (_table$options$featur2 = table.options.features).coreRowModel) === null || _table$options$featur === void 0 ? void 0 : _table$options$featur.call(_table$options$featur2, table)) ?? createCoreRowModel()(table);
  }
  return table._rowModels.coreRowModel();
}
function table_getPreFilteredRowModel(table) {
  return table.getCoreRowModel();
}
function table_getFilteredRowModel(table) {
  if (!table._rowModels.filteredRowModel) {
    var _table$options$featur3, _table$options$featur4;
    table._rowModels.filteredRowModel = (_table$options$featur3 = (_table$options$featur4 = table.options.features).filteredRowModel) === null || _table$options$featur3 === void 0 ? void 0 : _table$options$featur3.call(_table$options$featur4, table);
  }
  if (table.options.manualFiltering || !table._rowModels.filteredRowModel) return table.getPreFilteredRowModel();
  return table._rowModels.filteredRowModel();
}
function table_getPreGroupedRowModel(table) {
  return table.getFilteredRowModel();
}
function table_getGroupedRowModel(table) {
  if (!table._rowModels.groupedRowModel) {
    var _table$options$featur5, _table$options$featur6;
    table._rowModels.groupedRowModel = (_table$options$featur5 = (_table$options$featur6 = table.options.features).groupedRowModel) === null || _table$options$featur5 === void 0 ? void 0 : _table$options$featur5.call(_table$options$featur6, table);
  }
  if (table.options.manualGrouping || !table._rowModels.groupedRowModel) return table.getPreGroupedRowModel();
  return table._rowModels.groupedRowModel();
}
function table_getPreSortedRowModel(table) {
  return table.getGroupedRowModel();
}
function table_getSortedRowModel(table) {
  if (!table._rowModels.sortedRowModel) {
    var _table$options$featur7, _table$options$featur8;
    table._rowModels.sortedRowModel = (_table$options$featur7 = (_table$options$featur8 = table.options.features).sortedRowModel) === null || _table$options$featur7 === void 0 ? void 0 : _table$options$featur7.call(_table$options$featur8, table);
  }
  if (table.options.manualSorting || !table._rowModels.sortedRowModel) return table.getPreSortedRowModel();
  return table._rowModels.sortedRowModel();
}
function table_getPreExpandedRowModel(table) {
  return table.getSortedRowModel();
}
function table_getExpandedRowModel(table) {
  if (!table._rowModels.expandedRowModel) {
    var _table$options$featur9, _table$options$featur10;
    table._rowModels.expandedRowModel = (_table$options$featur9 = (_table$options$featur10 = table.options.features).expandedRowModel) === null || _table$options$featur9 === void 0 ? void 0 : _table$options$featur9.call(_table$options$featur10, table);
  }
  if (table.options.manualExpanding || !table._rowModels.expandedRowModel) return table.getPreExpandedRowModel();
  return table._rowModels.expandedRowModel();
}
function table_getPrePaginatedRowModel(table) {
  return table.getExpandedRowModel();
}
function table_getPaginatedRowModel(table) {
  if (!table._rowModels.paginatedRowModel) {
    var _table$options$featur11, _table$options$featur12;
    table._rowModels.paginatedRowModel = (_table$options$featur11 = (_table$options$featur12 = table.options.features).paginatedRowModel) === null || _table$options$featur11 === void 0 ? void 0 : _table$options$featur11.call(_table$options$featur12, table);
  }
  if (table.options.manualPagination || !table._rowModels.paginatedRowModel) return table.getPrePaginatedRowModel();
  return table._rowModels.paginatedRowModel();
}
function table_getRowModel(table) {
  return table.getPaginatedRowModel();
}

// node_modules/@tanstack/table-core/dist/core/row-models/coreRowModelsFeature.js
var coreRowModelsFeature = { constructTableAPIs: (table) => {
  assignTableAPIs("coreRowModelsFeature", table, {
    table_getCoreRowModel: { fn: () => table_getCoreRowModel(table) },
    table_getPreFilteredRowModel: { fn: () => table_getPreFilteredRowModel(table) },
    table_getFilteredRowModel: { fn: () => table_getFilteredRowModel(table) },
    table_getPreGroupedRowModel: { fn: () => table_getPreGroupedRowModel(table) },
    table_getGroupedRowModel: { fn: () => table_getGroupedRowModel(table) },
    table_getPreSortedRowModel: { fn: () => table_getPreSortedRowModel(table) },
    table_getSortedRowModel: { fn: () => table_getSortedRowModel(table) },
    table_getPreExpandedRowModel: { fn: () => table_getPreExpandedRowModel(table) },
    table_getExpandedRowModel: { fn: () => table_getExpandedRowModel(table) },
    table_getPrePaginatedRowModel: { fn: () => table_getPrePaginatedRowModel(table) },
    table_getPaginatedRowModel: { fn: () => table_getPaginatedRowModel(table) },
    table_getRowModel: { fn: () => table_getRowModel(table) }
  });
} };

// node_modules/@tanstack/table-core/dist/core/cells/constructCell.js
function getCellPrototype(table) {
  if (!table._cellPrototype) {
    table._cellPrototype = { table };
    const features = Object.values(table._features);
    for (let i6 = 0; i6 < features.length; i6++) {
      var _assignCellPrototype, _ref;
      (_assignCellPrototype = (_ref = features[i6]).assignCellPrototype) === null || _assignCellPrototype === void 0 || _assignCellPrototype.call(_ref, table._cellPrototype, table);
    }
  }
  return table._cellPrototype;
}
function constructCell(column, row, table) {
  const cellPrototype = getCellPrototype(table);
  const cell = Object.create(cellPrototype);
  cell.column = column;
  cell.id = `${row.id}_${column.id}`;
  cell.row = row;
  return cell;
}

// node_modules/@tanstack/table-core/dist/core/rows/coreRowsFeature.utils.js
function row_getValue(row, columnId) {
  if (row._valuesCache.hasOwnProperty(columnId)) return row._valuesCache[columnId];
  const column = row.table.getColumn(columnId);
  if (!(column === null || column === void 0 ? void 0 : column.accessorFn)) return;
  row._valuesCache[columnId] = column.accessorFn(row.original, row.index);
  return row._valuesCache[columnId];
}
function row_getUniqueValues(row, columnId) {
  if (row._uniqueValuesCache.hasOwnProperty(columnId)) return row._uniqueValuesCache[columnId];
  const column = row.table.getColumn(columnId);
  if (!(column === null || column === void 0 ? void 0 : column.accessorFn)) return;
  if (!column.columnDef.getUniqueValues) {
    row._uniqueValuesCache[columnId] = [row.getValue(columnId)];
    return row._uniqueValuesCache[columnId];
  }
  row._uniqueValuesCache[columnId] = column.columnDef.getUniqueValues(row.original, row.index);
  return row._uniqueValuesCache[columnId];
}
function row_renderValue(row, columnId) {
  return row.getValue(columnId) ?? row.table.options.renderFallbackValue;
}
function row_getLeafRows(row) {
  return flattenBy(row.subRows, (d3) => d3.subRows);
}
function row_getParentRow(row) {
  return row.parentId ? row.table.getRow(row.parentId, true) : void 0;
}
function row_getParentRows(row) {
  const parentRows = [];
  let currentRow = row;
  while (true) {
    const parentRow = currentRow.getParentRow();
    if (!parentRow) break;
    parentRows.push(parentRow);
    currentRow = parentRow;
  }
  return parentRows.reverse();
}
function row_getAllCells(row) {
  const columns = row.table.getAllLeafColumns();
  const cells = new Array(columns.length);
  for (let i6 = 0; i6 < columns.length; i6++) cells[i6] = constructCell(columns[i6], row, row.table);
  return cells;
}
function row_getAllCellsByColumnId(row) {
  const result = {};
  const cells = row.getAllCells();
  for (let i6 = 0; i6 < cells.length; i6++) {
    const cell = cells[i6];
    result[cell.column.id] = cell;
  }
  return result;
}
function table_getRowId(originalRow, table, index, parent) {
  var _table$options$getRow, _table$options;
  return ((_table$options$getRow = (_table$options = table.options).getRowId) === null || _table$options$getRow === void 0 ? void 0 : _table$options$getRow.call(_table$options, originalRow, index, parent)) ?? `${parent ? [parent.id, index].join(".") : index}`;
}
function table_getRow(table, rowId, searchAll) {
  let row = (searchAll ? table.getPrePaginatedRowModel() : table.getRowModel()).rowsById[rowId];
  if (!row) {
    row = table.getCoreRowModel().rowsById[rowId];
    if (!row) {
      if (true) throw new Error(`getRow could not find row with ID: ${rowId}`);
      throw new Error();
    }
  }
  return row;
}

// node_modules/@tanstack/table-core/dist/core/rows/coreRowsFeature.js
var coreRowsFeature = {
  assignRowPrototype: (prototype, table) => {
    assignPrototypeAPIs("coreRowsFeature", prototype, table, {
      row_getAllCellsByColumnId: {
        fn: (row) => row_getAllCellsByColumnId(row),
        memoDeps: (row) => [row.getAllCells()]
      },
      row_getAllCells: {
        fn: (row) => row_getAllCells(row),
        memoDeps: (row) => [row.table.getAllLeafColumns()]
      },
      row_getLeafRows: { fn: (row) => row_getLeafRows(row) },
      row_getParentRow: { fn: (row) => row_getParentRow(row) },
      row_getParentRows: { fn: (row) => row_getParentRows(row) },
      row_getUniqueValues: { fn: (row, columnId) => row_getUniqueValues(row, columnId) },
      row_getValue: { fn: (row, columnId) => row_getValue(row, columnId) },
      row_renderValue: { fn: (row, columnId) => row_renderValue(row, columnId) }
    });
  },
  constructTableAPIs: (table) => {
    assignTableAPIs("coreRowsFeature", table, {
      table_getRowId: { fn: (originalRow, index, parent) => table_getRowId(originalRow, table, index, parent) },
      table_getRow: { fn: (id, searchAll) => table_getRow(table, id, searchAll) }
    });
  }
};

// node_modules/@tanstack/table-core/dist/core/table/coreTablesFeature.utils.js
function table_syncExternalStateToBaseAtoms(table) {
  const state = table.options.state;
  if (!state) return;
  table._reactivity.batch(() => {
    for (const key in state) {
      const baseAtom = table.baseAtoms[key];
      if (!baseAtom) continue;
      const externalState = state[key];
      if (externalState !== baseAtom.get()) baseAtom.set(() => externalState);
    }
  });
}
function table_reset(table) {
  const snap = cloneState(table.initialState);
  table._reactivity.batch(() => {
    const keys = Object.keys(snap);
    for (let i6 = 0; i6 < keys.length; i6++) {
      const key = keys[i6];
      table.baseAtoms[key].set(snap[key]);
    }
  });
}
function table_mergeOptions(table, newOptions) {
  if (table.options.mergeOptions) return table.options.mergeOptions(table.options, newOptions);
  return {
    ...table.options,
    ...newOptions
  };
}
function table_setOptions(table, updater) {
  const newOptions = functionalUpdate(updater, table.options);
  const { features, atoms, initialState } = table.options;
  const mergedOptions = Object.assign(table_mergeOptions(table, newOptions), {
    features,
    atoms,
    initialState
  });
  if (table.optionsStore) table.optionsStore.set(() => mergedOptions);
  else table.options = mergedOptions;
  table_syncExternalStateToBaseAtoms(table);
}

// node_modules/@tanstack/table-core/dist/core/table/coreTablesFeature.js
var coreTablesFeature = { constructTableAPIs: (table) => {
  assignTableAPIs("coreTablesFeature", table, {
    table_reset: { fn: () => table_reset(table) },
    table_setOptions: { fn: (updater) => table_setOptions(table, updater) }
  });
} };

// node_modules/@tanstack/table-core/dist/core/coreFeatures.js
var coreFeatures = {
  coreCellsFeature,
  coreColumnsFeature,
  coreHeadersFeature,
  coreRowModelsFeature,
  coreRowsFeature,
  coreTablesFeature
};

// node_modules/@tanstack/table-core/dist/helpers/tableFeatures.js
function tableFeatures(features) {
  return features;
}

// node_modules/@tanstack/table-core/dist/core/reactivity/coreReactivityFeature.utils.js
function atomToStore(atom) {
  const store = atom;
  Object.defineProperty(atom, "state", { get() {
    return atom.get();
  } });
  if ("set" in atom) store.setState = atom.set.bind(atom);
  return store;
}

// node_modules/@tanstack/table-core/dist/core/table/constructTable.js
function getInitialTableState(features, initialState = {}) {
  Object.values(features).forEach((feature) => {
    var _feature$getInitialSt;
    initialState = ((_feature$getInitialSt = feature.getInitialState) === null || _feature$getInitialSt === void 0 ? void 0 : _feature$getInitialSt.call(feature, initialState)) ?? initialState;
  });
  return cloneState(initialState);
}
function constructTable(tableOptions) {
  const _reactivity = tableOptions.features.coreReactivityFeature;
  const { aggregationFns, columnMeta: _columnMeta, coreRowModel, expandedRowModel, facetedMinMaxValues, facetedRowModel, facetedUniqueValues, filterFns, filterMeta: _filterMeta, filteredRowModel, groupedRowModel, paginatedRowModel, sortFns, sortedRowModel, tableMeta: _tableMeta, ...features } = tableOptions.features;
  const table = {
    _reactivity,
    _features: {
      ...coreFeatures,
      ...features
    },
    _rowModels: {},
    _rowModelFns: {
      aggregationFns,
      filterFns,
      sortFns
    },
    baseAtoms: {},
    atoms: {}
  };
  const featuresList = Object.values(table._features);
  const mergedOptions = {
    ...featuresList.reduce((obj, feature) => {
      var _feature$getDefaultTa;
      return Object.assign(obj, (_feature$getDefaultTa = feature.getDefaultTableOptions) === null || _feature$getDefaultTa === void 0 ? void 0 : _feature$getDefaultTa.call(feature, table));
    }, {}),
    ...tableOptions
  };
  if (_reactivity.wrapExternalAtoms && mergedOptions.atoms) for (const [atomKey, _atom] of Object.entries(mergedOptions.atoms)) {
    const atom = _atom;
    const wrappedAtom = _reactivity.createWritableAtom(atom.get(), { debugName: `externalAtom/${atomKey}` });
    mergedOptions.atoms[atomKey] = wrappedAtom;
    let syncExternal = false;
    const syncAtomToWrappedSub = atom.subscribe((value) => {
      if (syncExternal) return;
      wrappedAtom.set(value);
    });
    const syncWrappedToAtomSub = wrappedAtom.subscribe((value) => {
      syncExternal = true;
      atom.set(value);
      syncExternal = false;
    });
    _reactivity.addSubscription(syncAtomToWrappedSub);
    _reactivity.addSubscription(syncWrappedToAtomSub);
  }
  if (_reactivity.createOptionsStore) {
    table.optionsStore = _reactivity.createWritableAtom(mergedOptions, { debugName: "table/optionsStore" });
    Object.defineProperty(table, "options", {
      configurable: true,
      enumerable: true,
      get() {
        return table.optionsStore.get();
      },
      set(value) {
        table.optionsStore.set(() => value);
      }
    });
  } else table.options = mergedOptions;
  table.initialState = getInitialTableState(table._features, table.options.initialState);
  const stateKeys = Object.keys(table.initialState);
  for (let i6 = 0; i6 < stateKeys.length; i6++) {
    const key = stateKeys[i6];
    table.baseAtoms[key] = _reactivity.createWritableAtom(table.initialState[key], { debugName: `table/baseAtoms/${key}` });
    table.atoms[key] = _reactivity.createReadonlyAtom(() => {
      const externalAtoms = table.options.atoms;
      const externalAtom = externalAtoms === null || externalAtoms === void 0 ? void 0 : externalAtoms[key];
      if (externalAtom) return externalAtom.get();
      return table.baseAtoms[key].get();
    }, { debugName: `table/atoms/${key}` });
  }
  table_syncExternalStateToBaseAtoms(table);
  table.store = atomToStore(_reactivity.createReadonlyAtom(() => {
    const snapshot = {};
    for (let i6 = 0; i6 < stateKeys.length; i6++) {
      const key = stateKeys[i6];
      snapshot[key] = table.atoms[key].get();
    }
    return snapshot;
  }, { debugName: "table/store" }));
  if (tableOptions.debugAll || tableOptions.debugTable) {
    const features2 = Object.keys(table._features);
    const rowModels = Object.entries({
      coreRowModel,
      filteredRowModel,
      groupedRowModel,
      sortedRowModel,
      expandedRowModel,
      paginatedRowModel,
      facetedRowModel,
      facetedMinMaxValues,
      facetedUniqueValues
    }).filter(([, factory]) => factory).map(([key]) => key);
    const states = Object.keys(table.initialState);
    console.log(`Constructing Table Instance

  Features:   ${features2.join("\n              ")}

  Row Models: ${rowModels.length ? rowModels.join("\n              ") : "(none)"}

  States:     ${states.join("\n              ")}
`, { table });
  }
  for (let i6 = 0; i6 < featuresList.length; i6++) {
    var _constructTableAPIs, _ref;
    (_constructTableAPIs = (_ref = featuresList[i6]).constructTableAPIs) === null || _constructTableAPIs === void 0 || _constructTableAPIs.call(_ref, table);
  }
  return table;
}

// node_modules/@tanstack/table-core/dist/fns/sortFns.js
var reSplitAlphaNumeric = /([0-9]+)/gm;
var sortFn_basic = (rowA, rowB, columnId) => {
  return compareBasic(rowA.getValue(columnId), rowB.getValue(columnId));
};
function compareBasic(a3, b3) {
  return a3 === b3 ? 0 : a3 > b3 ? 1 : -1;
}

// node_modules/@tanstack/table-core/dist/features/column-pinning/columnPinningFeature.js
var columnPinningFeature = {
  getInitialState: (initialState) => {
    return {
      columnPinning: {
        ...getDefaultColumnPinningState(),
        ...initialState.columnPinning
      },
      ...initialState
    };
  },
  getDefaultTableOptions: (table) => {
    return { onColumnPinningChange: makeStateUpdater("columnPinning", table) };
  },
  assignColumnPrototype: (prototype, table) => {
    assignPrototypeAPIs("columnPinningFeature", prototype, table, {
      column_pin: { fn: (column, position) => column_pin(column, position) },
      column_getCanPin: { fn: (column) => column_getCanPin(column) },
      column_getPinnedIndex: { fn: (column) => column_getPinnedIndex(column) },
      column_getIsPinned: { fn: (column) => column_getIsPinned(column) }
    });
  },
  assignRowPrototype: (prototype, table) => {
    assignPrototypeAPIs("columnPinningFeature", prototype, table, {
      row_getCenterVisibleCells: {
        fn: (row) => row_getCenterVisibleCells(row),
        memoDeps: (row) => {
          var _row$table$atoms$colu, _row$table$atoms$colu2;
          return [
            row.getAllCells(),
            (_row$table$atoms$colu = row.table.atoms.columnPinning) === null || _row$table$atoms$colu === void 0 ? void 0 : _row$table$atoms$colu.get(),
            (_row$table$atoms$colu2 = row.table.atoms.columnVisibility) === null || _row$table$atoms$colu2 === void 0 ? void 0 : _row$table$atoms$colu2.get()
          ];
        }
      },
      row_getLeftVisibleCells: {
        fn: (row) => row_getLeftVisibleCells(row),
        memoDeps: (row) => {
          var _row$table$atoms$colu3, _row$table$atoms$colu4;
          return [
            row.getAllCells(),
            (_row$table$atoms$colu3 = row.table.atoms.columnPinning) === null || _row$table$atoms$colu3 === void 0 || (_row$table$atoms$colu3 = _row$table$atoms$colu3.get()) === null || _row$table$atoms$colu3 === void 0 ? void 0 : _row$table$atoms$colu3.left,
            (_row$table$atoms$colu4 = row.table.atoms.columnVisibility) === null || _row$table$atoms$colu4 === void 0 ? void 0 : _row$table$atoms$colu4.get()
          ];
        }
      },
      row_getRightVisibleCells: {
        fn: (row) => row_getRightVisibleCells(row),
        memoDeps: (row) => {
          var _row$table$atoms$colu5, _row$table$atoms$colu6;
          return [
            row.getAllCells(),
            (_row$table$atoms$colu5 = row.table.atoms.columnPinning) === null || _row$table$atoms$colu5 === void 0 || (_row$table$atoms$colu5 = _row$table$atoms$colu5.get()) === null || _row$table$atoms$colu5 === void 0 ? void 0 : _row$table$atoms$colu5.right,
            (_row$table$atoms$colu6 = row.table.atoms.columnVisibility) === null || _row$table$atoms$colu6 === void 0 ? void 0 : _row$table$atoms$colu6.get()
          ];
        }
      }
    });
  },
  constructTableAPIs: (table) => {
    assignTableAPIs("columnPinningFeature", table, {
      table_setColumnPinning: { fn: (updater) => table_setColumnPinning(table, updater) },
      table_resetColumnPinning: { fn: (defaultState) => table_resetColumnPinning(table, defaultState) },
      table_getIsSomeColumnsPinned: { fn: (position) => table_getIsSomeColumnsPinned(table, position) },
      table_getLeftHeaderGroups: {
        fn: () => table_getLeftHeaderGroups(table),
        memoDeps: () => {
          var _table$atoms$columnPi, _table$atoms$columnOr;
          return [
            table.getAllColumns(),
            callMemoOrStaticFn(table, "getVisibleLeafColumns", table_getVisibleLeafColumns),
            (_table$atoms$columnPi = table.atoms.columnPinning) === null || _table$atoms$columnPi === void 0 || (_table$atoms$columnPi = _table$atoms$columnPi.get()) === null || _table$atoms$columnPi === void 0 ? void 0 : _table$atoms$columnPi.left,
            (_table$atoms$columnOr = table.atoms.columnOrder) === null || _table$atoms$columnOr === void 0 ? void 0 : _table$atoms$columnOr.get()
          ];
        }
      },
      table_getCenterHeaderGroups: {
        fn: () => table_getCenterHeaderGroups(table),
        memoDeps: () => {
          var _table$atoms$columnPi2, _table$atoms$columnOr2;
          return [
            table.getAllColumns(),
            callMemoOrStaticFn(table, "getVisibleLeafColumns", table_getVisibleLeafColumns),
            (_table$atoms$columnPi2 = table.atoms.columnPinning) === null || _table$atoms$columnPi2 === void 0 ? void 0 : _table$atoms$columnPi2.get(),
            (_table$atoms$columnOr2 = table.atoms.columnOrder) === null || _table$atoms$columnOr2 === void 0 ? void 0 : _table$atoms$columnOr2.get()
          ];
        }
      },
      table_getRightHeaderGroups: {
        fn: () => table_getRightHeaderGroups(table),
        memoDeps: () => {
          var _table$atoms$columnPi3, _table$atoms$columnOr3;
          return [
            table.getAllColumns(),
            callMemoOrStaticFn(table, "getVisibleLeafColumns", table_getVisibleLeafColumns),
            (_table$atoms$columnPi3 = table.atoms.columnPinning) === null || _table$atoms$columnPi3 === void 0 || (_table$atoms$columnPi3 = _table$atoms$columnPi3.get()) === null || _table$atoms$columnPi3 === void 0 ? void 0 : _table$atoms$columnPi3.right,
            (_table$atoms$columnOr3 = table.atoms.columnOrder) === null || _table$atoms$columnOr3 === void 0 ? void 0 : _table$atoms$columnOr3.get()
          ];
        }
      },
      table_getLeftFooterGroups: {
        fn: () => table_getLeftFooterGroups(table),
        memoDeps: () => [callMemoOrStaticFn(table, "getLeftHeaderGroups", table_getLeftHeaderGroups)]
      },
      table_getCenterFooterGroups: {
        fn: () => table_getCenterFooterGroups(table),
        memoDeps: () => [callMemoOrStaticFn(table, "getCenterHeaderGroups", table_getCenterHeaderGroups)]
      },
      table_getRightFooterGroups: {
        fn: () => table_getRightFooterGroups(table),
        memoDeps: () => [callMemoOrStaticFn(table, "getRightHeaderGroups", table_getRightHeaderGroups)]
      },
      table_getLeftFlatHeaders: {
        fn: () => table_getLeftFlatHeaders(table),
        memoDeps: () => [callMemoOrStaticFn(table, "getLeftHeaderGroups", table_getLeftHeaderGroups)]
      },
      table_getRightFlatHeaders: {
        fn: () => table_getRightFlatHeaders(table),
        memoDeps: () => [callMemoOrStaticFn(table, "getRightHeaderGroups", table_getRightHeaderGroups)]
      },
      table_getCenterFlatHeaders: {
        fn: () => table_getCenterFlatHeaders(table),
        memoDeps: () => [callMemoOrStaticFn(table, "getCenterHeaderGroups", table_getCenterHeaderGroups)]
      },
      table_getLeftLeafHeaders: {
        fn: () => table_getLeftLeafHeaders(table),
        memoDeps: () => [callMemoOrStaticFn(table, "getLeftHeaderGroups", table_getLeftHeaderGroups)]
      },
      table_getRightLeafHeaders: {
        fn: () => table_getRightLeafHeaders(table),
        memoDeps: () => [callMemoOrStaticFn(table, "getRightHeaderGroups", table_getRightHeaderGroups)]
      },
      table_getCenterLeafHeaders: {
        fn: () => table_getCenterLeafHeaders(table),
        memoDeps: () => [callMemoOrStaticFn(table, "getCenterHeaderGroups", table_getCenterHeaderGroups)]
      },
      table_getLeftLeafColumns: {
        fn: () => table_getLeftLeafColumns(table),
        memoDeps: () => {
          var _table$atoms$columnPi4, _table$atoms$columnOr4;
          return [
            table.options.columns,
            (_table$atoms$columnPi4 = table.atoms.columnPinning) === null || _table$atoms$columnPi4 === void 0 ? void 0 : _table$atoms$columnPi4.get(),
            (_table$atoms$columnOr4 = table.atoms.columnOrder) === null || _table$atoms$columnOr4 === void 0 ? void 0 : _table$atoms$columnOr4.get()
          ];
        }
      },
      table_getRightLeafColumns: {
        fn: () => table_getRightLeafColumns(table),
        memoDeps: () => {
          var _table$atoms$columnPi5, _table$atoms$columnOr5;
          return [
            table.options.columns,
            (_table$atoms$columnPi5 = table.atoms.columnPinning) === null || _table$atoms$columnPi5 === void 0 ? void 0 : _table$atoms$columnPi5.get(),
            (_table$atoms$columnOr5 = table.atoms.columnOrder) === null || _table$atoms$columnOr5 === void 0 ? void 0 : _table$atoms$columnOr5.get()
          ];
        }
      },
      table_getCenterLeafColumns: {
        fn: () => table_getCenterLeafColumns(table),
        memoDeps: () => {
          var _table$atoms$columnPi6, _table$atoms$columnOr6;
          return [
            table.options.columns,
            (_table$atoms$columnPi6 = table.atoms.columnPinning) === null || _table$atoms$columnPi6 === void 0 ? void 0 : _table$atoms$columnPi6.get(),
            (_table$atoms$columnOr6 = table.atoms.columnOrder) === null || _table$atoms$columnOr6 === void 0 ? void 0 : _table$atoms$columnOr6.get()
          ];
        }
      },
      table_getPinnedLeafColumns: { fn: (position) => table_getPinnedLeafColumns(table, position) },
      table_getLeftVisibleLeafColumns: {
        fn: () => table_getLeftVisibleLeafColumns(table),
        memoDeps: () => {
          var _table$atoms$columnPi7, _table$atoms$columnVi, _table$atoms$columnOr7;
          return [
            table.options.columns,
            (_table$atoms$columnPi7 = table.atoms.columnPinning) === null || _table$atoms$columnPi7 === void 0 ? void 0 : _table$atoms$columnPi7.get(),
            (_table$atoms$columnVi = table.atoms.columnVisibility) === null || _table$atoms$columnVi === void 0 ? void 0 : _table$atoms$columnVi.get(),
            (_table$atoms$columnOr7 = table.atoms.columnOrder) === null || _table$atoms$columnOr7 === void 0 ? void 0 : _table$atoms$columnOr7.get()
          ];
        }
      },
      table_getCenterVisibleLeafColumns: {
        fn: () => table_getCenterVisibleLeafColumns(table),
        memoDeps: () => {
          var _table$atoms$columnPi8, _table$atoms$columnVi2, _table$atoms$columnOr8;
          return [
            table.options.columns,
            (_table$atoms$columnPi8 = table.atoms.columnPinning) === null || _table$atoms$columnPi8 === void 0 ? void 0 : _table$atoms$columnPi8.get(),
            (_table$atoms$columnVi2 = table.atoms.columnVisibility) === null || _table$atoms$columnVi2 === void 0 ? void 0 : _table$atoms$columnVi2.get(),
            (_table$atoms$columnOr8 = table.atoms.columnOrder) === null || _table$atoms$columnOr8 === void 0 ? void 0 : _table$atoms$columnOr8.get()
          ];
        }
      },
      table_getRightVisibleLeafColumns: {
        fn: () => table_getRightVisibleLeafColumns(table),
        memoDeps: () => {
          var _table$atoms$columnPi9, _table$atoms$columnVi3, _table$atoms$columnOr9;
          return [
            table.options.columns,
            (_table$atoms$columnPi9 = table.atoms.columnPinning) === null || _table$atoms$columnPi9 === void 0 ? void 0 : _table$atoms$columnPi9.get(),
            (_table$atoms$columnVi3 = table.atoms.columnVisibility) === null || _table$atoms$columnVi3 === void 0 ? void 0 : _table$atoms$columnVi3.get(),
            (_table$atoms$columnOr9 = table.atoms.columnOrder) === null || _table$atoms$columnOr9 === void 0 ? void 0 : _table$atoms$columnOr9.get()
          ];
        }
      },
      table_getPinnedVisibleLeafColumns: { fn: (position) => table_getPinnedVisibleLeafColumns(table, position) }
    });
  }
};

// node_modules/@tanstack/table-core/dist/features/column-sizing/columnSizingFeature.utils.js
function getDefaultColumnSizingState() {
  return {};
}
function getDefaultColumnSizingColumnDef() {
  return {
    size: 150,
    minSize: 20,
    maxSize: Number.MAX_SAFE_INTEGER
  };
}
function column_getSize(column) {
  var _column$table$atoms$c;
  const defaultSizes = getDefaultColumnSizingColumnDef();
  const columnSize = (_column$table$atoms$c = column.table.atoms.columnSizing) === null || _column$table$atoms$c === void 0 || (_column$table$atoms$c = _column$table$atoms$c.get()) === null || _column$table$atoms$c === void 0 ? void 0 : _column$table$atoms$c[column.id];
  return Math.min(Math.max(column.columnDef.minSize ?? defaultSizes.minSize, columnSize ?? column.columnDef.size ?? defaultSizes.size), column.columnDef.maxSize ?? defaultSizes.maxSize);
}
function column_getStart(column, position) {
  const index = callMemoOrStaticFn(column, "getIndex", column_getIndex, position);
  if (index <= 0) return 0;
  const prevColumn = callMemoOrStaticFn(column.table, "getPinnedVisibleLeafColumns", table_getPinnedVisibleLeafColumns, position)[index - 1];
  return callMemoOrStaticFn(prevColumn, "getStart", column_getStart, position) + callMemoOrStaticFn(prevColumn, "getSize", column_getSize);
}
function column_getAfter(column, position) {
  const visibleLeafColumns = callMemoOrStaticFn(column.table, "getPinnedVisibleLeafColumns", table_getPinnedVisibleLeafColumns, position);
  const index = callMemoOrStaticFn(column, "getIndex", column_getIndex, position);
  if (index < 0 || index >= visibleLeafColumns.length - 1) return 0;
  const nextColumn = visibleLeafColumns[index + 1];
  return callMemoOrStaticFn(nextColumn, "getSize", column_getSize) + callMemoOrStaticFn(nextColumn, "getAfter", column_getAfter, position);
}
function column_resetSize(column) {
  table_setColumnSizing(column.table, ({ [column.id]: _2, ...rest }) => {
    return rest;
  });
}
function header_getSize(header) {
  let sum = 0;
  const recurse = (h5) => {
    if (h5.subHeaders.length) h5.subHeaders.forEach(recurse);
    else sum += column_getSize(h5.column);
  };
  recurse(header);
  return sum;
}
function header_getStart(header) {
  if (header.index > 0) {
    var _header$headerGroup;
    const prevSiblingHeader = (_header$headerGroup = header.headerGroup) === null || _header$headerGroup === void 0 ? void 0 : _header$headerGroup.headers[header.index - 1];
    if (prevSiblingHeader) return callMemoOrStaticFn(prevSiblingHeader, "getStart", header_getStart) + callMemoOrStaticFn(prevSiblingHeader, "getSize", header_getSize);
  }
  return 0;
}
function table_setColumnSizing(table, updater) {
  var _table$options$onColu, _table$options;
  (_table$options$onColu = (_table$options = table.options).onColumnSizingChange) === null || _table$options$onColu === void 0 || _table$options$onColu.call(_table$options, updater);
}
function table_resetColumnSizing(table, defaultState) {
  table_setColumnSizing(table, defaultState ? {} : cloneState(table.initialState.columnSizing ?? {}));
}
function table_getTotalSize(table) {
  var _table$getHeaderGroup;
  return ((_table$getHeaderGroup = table.getHeaderGroups()[0]) === null || _table$getHeaderGroup === void 0 ? void 0 : _table$getHeaderGroup.headers.reduce((sum, header) => {
    return sum + header_getSize(header);
  }, 0)) ?? 0;
}
function table_getLeftTotalSize(table) {
  var _callMemoOrStaticFn$;
  return ((_callMemoOrStaticFn$ = callMemoOrStaticFn(table, "getLeftHeaderGroups", table_getLeftHeaderGroups)[0]) === null || _callMemoOrStaticFn$ === void 0 ? void 0 : _callMemoOrStaticFn$.headers.reduce((sum, header) => {
    return sum + header_getSize(header);
  }, 0)) ?? 0;
}
function table_getCenterTotalSize(table) {
  var _callMemoOrStaticFn$2;
  return ((_callMemoOrStaticFn$2 = callMemoOrStaticFn(table, "getCenterHeaderGroups", table_getCenterHeaderGroups)[0]) === null || _callMemoOrStaticFn$2 === void 0 ? void 0 : _callMemoOrStaticFn$2.headers.reduce((sum, header) => {
    return sum + header_getSize(header);
  }, 0)) ?? 0;
}
function table_getRightTotalSize(table) {
  var _callMemoOrStaticFn$3;
  return ((_callMemoOrStaticFn$3 = callMemoOrStaticFn(table, "getRightHeaderGroups", table_getRightHeaderGroups)[0]) === null || _callMemoOrStaticFn$3 === void 0 ? void 0 : _callMemoOrStaticFn$3.headers.reduce((sum, header) => {
    return sum + header_getSize(header);
  }, 0)) ?? 0;
}

// node_modules/@tanstack/table-core/dist/features/column-resizing/columnResizingFeature.utils.js
function getDefaultColumnResizingState() {
  return {
    startOffset: null,
    startSize: null,
    deltaOffset: null,
    deltaPercentage: null,
    isResizingColumn: false,
    columnSizingStart: []
  };
}
function column_getCanResize(column) {
  return (column.columnDef.enableResizing ?? true) && (column.table.options.enableColumnResizing ?? true);
}
function column_getIsResizing(column) {
  var _column$table$atoms$c;
  return ((_column$table$atoms$c = column.table.atoms.columnResizing) === null || _column$table$atoms$c === void 0 || (_column$table$atoms$c = _column$table$atoms$c.get()) === null || _column$table$atoms$c === void 0 ? void 0 : _column$table$atoms$c.isResizingColumn) === column.id;
}
function header_getResizeHandler(header, _contextDocument) {
  const column = header.table.getColumn(header.column.id);
  const canResize = column_getCanResize(column);
  return (event) => {
    var _persist;
    if (!canResize) return;
    (_persist = event.persist) === null || _persist === void 0 || _persist.call(event);
    if (isTouchStartEvent(event)) {
      if (event.touches.length > 1) return;
    }
    const startSize = header_getSize(header);
    const columnSizingStart = header.getLeafHeaders().map((leafHeader) => [leafHeader.column.id, column_getSize(leafHeader.column)]);
    const clientX = isTouchStartEvent(event) ? Math.round(event.touches[0].clientX) : event.clientX;
    const newColumnSizing = {};
    const updateOffset = (eventType, clientXPos) => {
      if (typeof clientXPos !== "number") return;
      table_setColumnResizing(column.table, (old) => {
        const deltaDirection = column.table.options.columnResizeDirection === "rtl" ? -1 : 1;
        const deltaOffset = (clientXPos - (old.startOffset ?? 0)) * deltaDirection;
        const startSize2 = old.startSize ?? 0;
        const deltaPercentage = Math.max(startSize2 > 0 ? deltaOffset / startSize2 : 0, -0.999999);
        old.columnSizingStart.forEach(([columnId, headerSize]) => {
          newColumnSizing[columnId] = Math.round(Math.max(headerSize > 0 ? headerSize + headerSize * deltaPercentage : deltaOffset / old.columnSizingStart.length, 0) * 100) / 100;
        });
        return {
          ...old,
          deltaOffset,
          deltaPercentage
        };
      });
      if (column.table.options.columnResizeMode === "onChange" || eventType === "end") table_setColumnSizing(column.table, (old) => ({
        ...old,
        ...newColumnSizing
      }));
    };
    const onMove = (clientXPos) => updateOffset("move", clientXPos);
    const onEnd = (clientXPos) => {
      updateOffset("end", clientXPos);
      table_setColumnResizing(column.table, (old) => ({
        ...old,
        isResizingColumn: false,
        startOffset: null,
        startSize: null,
        deltaOffset: null,
        deltaPercentage: null,
        columnSizingStart: []
      }));
    };
    const contextDocument = _contextDocument || typeof document !== "undefined" ? document : null;
    const mouseEvents = {
      moveHandler: (e6) => onMove(e6.clientX),
      upHandler: (e6) => {
        contextDocument === null || contextDocument === void 0 || contextDocument.removeEventListener("mousemove", mouseEvents.moveHandler);
        contextDocument === null || contextDocument === void 0 || contextDocument.removeEventListener("mouseup", mouseEvents.upHandler);
        onEnd(e6.clientX);
      }
    };
    const touchEvents = {
      moveHandler: (touchEvent) => {
        if (touchEvent.cancelable) {
          touchEvent.preventDefault();
          touchEvent.stopPropagation();
        }
        onMove(touchEvent.touches[0].clientX);
        return false;
      },
      upHandler: (e6) => {
        var _e$touches$;
        contextDocument === null || contextDocument === void 0 || contextDocument.removeEventListener("touchmove", touchEvents.moveHandler);
        contextDocument === null || contextDocument === void 0 || contextDocument.removeEventListener("touchend", touchEvents.upHandler);
        if (e6.cancelable) {
          e6.preventDefault();
          e6.stopPropagation();
        }
        onEnd((_e$touches$ = e6.touches[0]) === null || _e$touches$ === void 0 ? void 0 : _e$touches$.clientX);
      }
    };
    const passiveIfSupported = passiveEventSupported() ? { passive: false } : false;
    if (isTouchStartEvent(event)) {
      contextDocument === null || contextDocument === void 0 || contextDocument.addEventListener("touchmove", touchEvents.moveHandler, passiveIfSupported);
      contextDocument === null || contextDocument === void 0 || contextDocument.addEventListener("touchend", touchEvents.upHandler, passiveIfSupported);
    } else {
      contextDocument === null || contextDocument === void 0 || contextDocument.addEventListener("mousemove", mouseEvents.moveHandler, passiveIfSupported);
      contextDocument === null || contextDocument === void 0 || contextDocument.addEventListener("mouseup", mouseEvents.upHandler, passiveIfSupported);
    }
    table_setColumnResizing(column.table, (old) => ({
      ...old,
      startOffset: clientX,
      startSize,
      deltaOffset: 0,
      deltaPercentage: 0,
      columnSizingStart,
      isResizingColumn: column.id
    }));
  };
}
function table_setColumnResizing(table, updater) {
  var _table$options$onColu, _table$options;
  (_table$options$onColu = (_table$options = table.options).onColumnResizingChange) === null || _table$options$onColu === void 0 || _table$options$onColu.call(_table$options, updater);
}
function table_resetHeaderSizeInfo(table, defaultState) {
  table_setColumnResizing(table, defaultState ? getDefaultColumnResizingState() : cloneState(table.initialState.columnResizing ?? getDefaultColumnResizingState()));
}
var passiveSupported = null;
function passiveEventSupported() {
  if (typeof passiveSupported === "boolean") return passiveSupported;
  let supported = false;
  try {
    const options = { get passive() {
      supported = true;
      return false;
    } };
    const noop = () => {
    };
    window.addEventListener("test", noop, options);
    window.removeEventListener("test", noop);
  } catch (err) {
    supported = false;
  }
  passiveSupported = supported;
  return passiveSupported;
}
function isTouchStartEvent(e6) {
  return e6.type === "touchstart";
}

// node_modules/@tanstack/table-core/dist/features/column-resizing/columnResizingFeature.js
var columnResizingFeature = {
  getInitialState: (initialState) => {
    return {
      columnResizing: getDefaultColumnResizingState(),
      ...initialState
    };
  },
  getDefaultTableOptions: (table) => {
    return {
      columnResizeMode: "onEnd",
      columnResizeDirection: "ltr",
      onColumnResizingChange: makeStateUpdater("columnResizing", table)
    };
  },
  assignColumnPrototype: (prototype, table) => {
    assignPrototypeAPIs("columnResizingFeature", prototype, table, {
      column_getCanResize: { fn: (column) => column_getCanResize(column) },
      column_getIsResizing: { fn: (column) => column_getIsResizing(column) }
    });
  },
  assignHeaderPrototype: (prototype, table) => {
    assignPrototypeAPIs("columnResizingFeature", prototype, table, { header_getResizeHandler: { fn: (header, _contextDocument) => header_getResizeHandler(header, _contextDocument) } });
  },
  constructTableAPIs: (table) => {
    assignTableAPIs("columnResizingFeature", table, {
      table_setColumnResizing: { fn: (updater) => table_setColumnResizing(table, updater) },
      table_resetHeaderSizeInfo: { fn: (defaultState) => table_resetHeaderSizeInfo(table, defaultState) }
    });
  }
};

// node_modules/@tanstack/table-core/dist/features/column-sizing/columnSizingFeature.js
var columnSizingFeature = {
  getInitialState: (initialState) => {
    return {
      columnSizing: getDefaultColumnSizingState(),
      ...initialState
    };
  },
  getDefaultColumnDef: () => {
    return getDefaultColumnSizingColumnDef();
  },
  getDefaultTableOptions: (table) => {
    return { onColumnSizingChange: makeStateUpdater("columnSizing", table) };
  },
  assignColumnPrototype: (prototype, table) => {
    assignPrototypeAPIs("columnSizingFeature", prototype, table, {
      column_getSize: {
        fn: (column) => column_getSize(column),
        memoDeps: (column) => {
          var _table$atoms$columnSi;
          return [table.options.columns, (_table$atoms$columnSi = table.atoms.columnSizing) === null || _table$atoms$columnSi === void 0 || (_table$atoms$columnSi = _table$atoms$columnSi.get()) === null || _table$atoms$columnSi === void 0 ? void 0 : _table$atoms$columnSi[column.id]];
        }
      },
      column_getStart: {
        fn: (column, position) => column_getStart(column, position),
        memoDeps: (column, position) => {
          var _table$atoms$columnSi2, _table$atoms$columnOr, _table$atoms$columnPi, _table$atoms$columnVi;
          return [
            position,
            table.options.columns,
            (_table$atoms$columnSi2 = table.atoms.columnSizing) === null || _table$atoms$columnSi2 === void 0 ? void 0 : _table$atoms$columnSi2.get(),
            (_table$atoms$columnOr = table.atoms.columnOrder) === null || _table$atoms$columnOr === void 0 ? void 0 : _table$atoms$columnOr.get(),
            (_table$atoms$columnPi = table.atoms.columnPinning) === null || _table$atoms$columnPi === void 0 ? void 0 : _table$atoms$columnPi.get(),
            (_table$atoms$columnVi = table.atoms.columnVisibility) === null || _table$atoms$columnVi === void 0 ? void 0 : _table$atoms$columnVi.get()
          ];
        }
      },
      column_getAfter: {
        fn: (column, position) => column_getAfter(column, position),
        memoDeps: (column, position) => {
          var _table$atoms$columnSi3, _table$atoms$columnOr2, _table$atoms$columnPi2, _table$atoms$columnVi2;
          return [
            position,
            table.options.columns,
            (_table$atoms$columnSi3 = table.atoms.columnSizing) === null || _table$atoms$columnSi3 === void 0 ? void 0 : _table$atoms$columnSi3.get(),
            (_table$atoms$columnOr2 = table.atoms.columnOrder) === null || _table$atoms$columnOr2 === void 0 ? void 0 : _table$atoms$columnOr2.get(),
            (_table$atoms$columnPi2 = table.atoms.columnPinning) === null || _table$atoms$columnPi2 === void 0 ? void 0 : _table$atoms$columnPi2.get(),
            (_table$atoms$columnVi2 = table.atoms.columnVisibility) === null || _table$atoms$columnVi2 === void 0 ? void 0 : _table$atoms$columnVi2.get()
          ];
        }
      },
      column_resetSize: { fn: (column) => column_resetSize(column) }
    });
  },
  assignHeaderPrototype: (prototype, table) => {
    assignPrototypeAPIs("columnSizingFeature", prototype, table, {
      header_getSize: {
        fn: (header) => header_getSize(header),
        memoDeps: (header) => {
          var _table$atoms$columnSi4, _table$atoms$columnSi5;
          return [table.options.columns, header.column.columns.length > 0 ? (_table$atoms$columnSi4 = table.atoms.columnSizing) === null || _table$atoms$columnSi4 === void 0 ? void 0 : _table$atoms$columnSi4.get() : (_table$atoms$columnSi5 = table.atoms.columnSizing) === null || _table$atoms$columnSi5 === void 0 || (_table$atoms$columnSi5 = _table$atoms$columnSi5.get()) === null || _table$atoms$columnSi5 === void 0 ? void 0 : _table$atoms$columnSi5[header.column.id]];
        }
      },
      header_getStart: {
        fn: (header) => header_getStart(header),
        memoDeps: (header, position) => {
          var _table$atoms$columnSi6, _table$atoms$columnOr3, _table$atoms$columnPi3, _table$atoms$columnVi3;
          return [
            position,
            table.options.columns,
            (_table$atoms$columnSi6 = table.atoms.columnSizing) === null || _table$atoms$columnSi6 === void 0 ? void 0 : _table$atoms$columnSi6.get(),
            (_table$atoms$columnOr3 = table.atoms.columnOrder) === null || _table$atoms$columnOr3 === void 0 ? void 0 : _table$atoms$columnOr3.get(),
            (_table$atoms$columnPi3 = table.atoms.columnPinning) === null || _table$atoms$columnPi3 === void 0 ? void 0 : _table$atoms$columnPi3.get(),
            (_table$atoms$columnVi3 = table.atoms.columnVisibility) === null || _table$atoms$columnVi3 === void 0 ? void 0 : _table$atoms$columnVi3.get()
          ];
        }
      }
    });
  },
  constructTableAPIs: (table) => {
    assignTableAPIs("columnSizingFeature", table, {
      table_setColumnSizing: { fn: (updater) => table_setColumnSizing(table, updater) },
      table_resetColumnSizing: { fn: (defaultState) => table_resetColumnSizing(table, defaultState) },
      table_getTotalSize: {
        fn: () => table_getTotalSize(table),
        memoDeps: () => {
          var _table$atoms$columnSi7;
          return [(_table$atoms$columnSi7 = table.atoms.columnSizing) === null || _table$atoms$columnSi7 === void 0 ? void 0 : _table$atoms$columnSi7.get(), table.getHeaderGroups()];
        }
      },
      table_getLeftTotalSize: {
        fn: () => table_getLeftTotalSize(table),
        memoDeps: () => {
          var _table$atoms$columnSi8;
          return [(_table$atoms$columnSi8 = table.atoms.columnSizing) === null || _table$atoms$columnSi8 === void 0 ? void 0 : _table$atoms$columnSi8.get(), table.getHeaderGroups()];
        }
      },
      table_getCenterTotalSize: {
        fn: () => table_getCenterTotalSize(table),
        memoDeps: () => {
          var _table$atoms$columnSi9;
          return [(_table$atoms$columnSi9 = table.atoms.columnSizing) === null || _table$atoms$columnSi9 === void 0 ? void 0 : _table$atoms$columnSi9.get(), table.getHeaderGroups()];
        }
      },
      table_getRightTotalSize: {
        fn: () => table_getRightTotalSize(table),
        memoDeps: () => {
          var _table$atoms$columnSi10;
          return [(_table$atoms$columnSi10 = table.atoms.columnSizing) === null || _table$atoms$columnSi10 === void 0 ? void 0 : _table$atoms$columnSi10.get(), table.getHeaderGroups()];
        }
      }
    });
  }
};

// node_modules/@tanstack/table-core/dist/features/column-visibility/columnVisibilityFeature.js
var columnVisibilityFeature = {
  getInitialState: (initialState) => {
    return {
      columnVisibility: getDefaultColumnVisibilityState(),
      ...initialState
    };
  },
  getDefaultTableOptions: (table) => {
    return { onColumnVisibilityChange: makeStateUpdater("columnVisibility", table) };
  },
  assignColumnPrototype: (prototype, table) => {
    assignPrototypeAPIs("columnVisibilityFeature", prototype, table, {
      column_getIsVisible: {
        fn: (column) => column_getIsVisible(column),
        memoDeps: (column) => {
          var _table$atoms$columnVi;
          return [
            table.options.columns,
            (_table$atoms$columnVi = table.atoms.columnVisibility) === null || _table$atoms$columnVi === void 0 ? void 0 : _table$atoms$columnVi.get(),
            column.columns
          ];
        }
      },
      column_getCanHide: { fn: (column) => column_getCanHide(column) },
      column_getToggleVisibilityHandler: { fn: (column) => column_getToggleVisibilityHandler(column) },
      column_toggleVisibility: { fn: (column, visible) => column_toggleVisibility(column, visible) }
    });
  },
  assignRowPrototype: (prototype, table) => {
    assignPrototypeAPIs("columnVisibilityFeature", prototype, table, {
      row_getVisibleCells: {
        fn: (row) => row_getVisibleCells(row),
        memoDeps: (row) => {
          var _table$atoms$columnPi, _table$atoms$columnVi2;
          return [
            row.getAllCells(),
            (_table$atoms$columnPi = table.atoms.columnPinning) === null || _table$atoms$columnPi === void 0 ? void 0 : _table$atoms$columnPi.get(),
            (_table$atoms$columnVi2 = table.atoms.columnVisibility) === null || _table$atoms$columnVi2 === void 0 ? void 0 : _table$atoms$columnVi2.get()
          ];
        }
      },
      row_getVisibleCellsByColumnId: {
        fn: (row) => row_getVisibleCellsByColumnId(row),
        memoDeps: (row) => {
          var _table$atoms$columnVi3;
          return [row.getAllCells(), (_table$atoms$columnVi3 = table.atoms.columnVisibility) === null || _table$atoms$columnVi3 === void 0 ? void 0 : _table$atoms$columnVi3.get()];
        }
      }
    });
  },
  constructTableAPIs: (table) => {
    assignTableAPIs("columnVisibilityFeature", table, {
      table_getVisibleFlatColumns: {
        fn: () => table_getVisibleFlatColumns(table),
        memoDeps: () => {
          var _table$atoms$columnVi4, _table$atoms$columnOr;
          return [
            (_table$atoms$columnVi4 = table.atoms.columnVisibility) === null || _table$atoms$columnVi4 === void 0 ? void 0 : _table$atoms$columnVi4.get(),
            (_table$atoms$columnOr = table.atoms.columnOrder) === null || _table$atoms$columnOr === void 0 ? void 0 : _table$atoms$columnOr.get(),
            table.options.columns
          ];
        }
      },
      table_getVisibleLeafColumns: {
        fn: () => table_getVisibleLeafColumns(table),
        memoDeps: () => {
          var _table$atoms$columnVi5, _table$atoms$columnOr2;
          return [
            (_table$atoms$columnVi5 = table.atoms.columnVisibility) === null || _table$atoms$columnVi5 === void 0 ? void 0 : _table$atoms$columnVi5.get(),
            (_table$atoms$columnOr2 = table.atoms.columnOrder) === null || _table$atoms$columnOr2 === void 0 ? void 0 : _table$atoms$columnOr2.get(),
            table.options.columns
          ];
        }
      },
      table_setColumnVisibility: { fn: (updater) => table_setColumnVisibility(table, updater) },
      table_resetColumnVisibility: { fn: (defaultState) => table_resetColumnVisibility(table, defaultState) },
      table_toggleAllColumnsVisible: { fn: (value) => table_toggleAllColumnsVisible(table, value) },
      table_getIsAllColumnsVisible: { fn: () => table_getIsAllColumnsVisible(table) },
      table_getIsSomeColumnsVisible: { fn: () => table_getIsSomeColumnsVisible(table) },
      table_getToggleAllColumnsVisibilityHandler: { fn: () => table_getToggleAllColumnsVisibilityHandler(table) }
    });
  }
};

// node_modules/@tanstack/table-core/dist/features/row-selection/rowSelectionFeature.utils.js
function getDefaultRowSelectionState() {
  return {};
}
function table_setRowSelection(table, updater) {
  var _table$options$onRowS, _table$options;
  (_table$options$onRowS = (_table$options = table.options).onRowSelectionChange) === null || _table$options$onRowS === void 0 || _table$options$onRowS.call(_table$options, updater);
}
function table_resetRowSelection(table, defaultState) {
  table_setRowSelection(table, defaultState ? {} : cloneState(table.initialState.rowSelection ?? {}));
}
function table_toggleAllRowsSelected(table, value) {
  table_setRowSelection(table, (old) => {
    value = typeof value !== "undefined" ? value : !table_getIsAllRowsSelected(table);
    const rowSelection = { ...old };
    const preGroupedFlatRows = table.getPreGroupedRowModel().flatRows;
    if (value) preGroupedFlatRows.forEach((row) => {
      if (!row_getCanSelect(row)) return;
      rowSelection[row.id] = true;
    });
    else preGroupedFlatRows.forEach((row) => {
      delete rowSelection[row.id];
    });
    return rowSelection;
  });
}
function table_toggleAllPageRowsSelected(table, value) {
  table_setRowSelection(table, (old) => {
    const resolvedValue = typeof value !== "undefined" ? value : !table_getIsAllPageRowsSelected(table);
    const rowSelection = { ...old };
    table.getRowModel().rows.forEach((row) => {
      mutateRowIsSelected(rowSelection, row.id, resolvedValue, true, table);
    });
    return rowSelection;
  });
}
function table_getPreSelectedRowModel(table) {
  return table.getCoreRowModel();
}
function table_getSelectedRowModel(table) {
  var _table$atoms$rowSelec;
  const rowModel = table.getCoreRowModel();
  if (!Object.keys(((_table$atoms$rowSelec = table.atoms.rowSelection) === null || _table$atoms$rowSelec === void 0 ? void 0 : _table$atoms$rowSelec.get()) ?? {}).length) return {
    rows: [],
    flatRows: [],
    rowsById: {}
  };
  return selectRowsFn(rowModel);
}
function table_getFilteredSelectedRowModel(table) {
  var _table$atoms$rowSelec2;
  const rowModel = table.getCoreRowModel();
  if (!Object.keys(((_table$atoms$rowSelec2 = table.atoms.rowSelection) === null || _table$atoms$rowSelec2 === void 0 ? void 0 : _table$atoms$rowSelec2.get()) ?? {}).length) return {
    rows: [],
    flatRows: [],
    rowsById: {}
  };
  return selectRowsFn(rowModel);
}
function table_getGroupedSelectedRowModel(table) {
  var _table$atoms$rowSelec3;
  const rowModel = table.getCoreRowModel();
  if (!Object.keys(((_table$atoms$rowSelec3 = table.atoms.rowSelection) === null || _table$atoms$rowSelec3 === void 0 ? void 0 : _table$atoms$rowSelec3.get()) ?? {}).length) return {
    rows: [],
    flatRows: [],
    rowsById: {}
  };
  return selectRowsFn(rowModel);
}
function table_getIsAllRowsSelected(table) {
  var _table$atoms$rowSelec4;
  const preGroupedFlatRows = table.getFilteredRowModel().flatRows;
  const rowSelection = ((_table$atoms$rowSelec4 = table.atoms.rowSelection) === null || _table$atoms$rowSelec4 === void 0 ? void 0 : _table$atoms$rowSelec4.get()) ?? {};
  let isAllRowsSelected = Boolean(preGroupedFlatRows.length && Object.keys(rowSelection).length);
  if (isAllRowsSelected) {
    if (preGroupedFlatRows.some((row) => row_getCanSelect(row) && !rowSelection[row.id])) isAllRowsSelected = false;
  }
  return isAllRowsSelected;
}
function table_getIsAllPageRowsSelected(table) {
  var _table$atoms$rowSelec5;
  const paginationFlatRows = table.getPaginatedRowModel().flatRows.filter((row) => row_getCanSelect(row));
  const rowSelection = ((_table$atoms$rowSelec5 = table.atoms.rowSelection) === null || _table$atoms$rowSelec5 === void 0 ? void 0 : _table$atoms$rowSelec5.get()) ?? {};
  let isAllPageRowsSelected = !!paginationFlatRows.length;
  if (isAllPageRowsSelected && paginationFlatRows.some((row) => !rowSelection[row.id])) isAllPageRowsSelected = false;
  return isAllPageRowsSelected;
}
function table_getIsSomeRowsSelected(table) {
  var _table$atoms$rowSelec6;
  const totalSelected = Object.keys(((_table$atoms$rowSelec6 = table.atoms.rowSelection) === null || _table$atoms$rowSelec6 === void 0 ? void 0 : _table$atoms$rowSelec6.get()) ?? {}).length;
  return totalSelected > 0 && totalSelected < table.getFilteredRowModel().flatRows.length;
}
function table_getIsSomePageRowsSelected(table) {
  const paginationFlatRows = table.getPaginatedRowModel().flatRows;
  return table_getIsAllPageRowsSelected(table) ? false : paginationFlatRows.filter((row) => row_getCanSelect(row)).some((row) => row_getIsSelected(row) || row_getIsSomeSelected(row));
}
function table_getToggleAllRowsSelectedHandler(table) {
  return (e6) => {
    table_toggleAllRowsSelected(table, e6.target.checked);
  };
}
function table_getToggleAllPageRowsSelectedHandler(table) {
  return (e6) => {
    table_toggleAllPageRowsSelected(table, e6.target.checked);
  };
}
function row_toggleSelected(row, value, opts) {
  const isSelected = row_getIsSelected(row);
  table_setRowSelection(row.table, (old) => {
    value = typeof value !== "undefined" ? value : !isSelected;
    if (row_getCanSelect(row) && isSelected === value) return old;
    const selectedRowIds = { ...old };
    mutateRowIsSelected(selectedRowIds, row.id, value, (opts === null || opts === void 0 ? void 0 : opts.selectChildren) ?? true, row.table);
    return selectedRowIds;
  });
}
function row_getIsSelected(row) {
  return isRowSelected(row);
}
function row_getIsSomeSelected(row) {
  return isSubRowSelected(row) === "some";
}
function row_getIsAllSubRowsSelected(row) {
  return isSubRowSelected(row) === "all";
}
function row_getCanSelect(row) {
  const options = row.table.options;
  if (typeof options.enableRowSelection === "function") return options.enableRowSelection(row);
  return options.enableRowSelection ?? true;
}
function row_getCanSelectSubRows(row) {
  const options = row.table.options;
  if (typeof options.enableSubRowSelection === "function") return options.enableSubRowSelection(row);
  return options.enableSubRowSelection ?? true;
}
function row_getCanMultiSelect(row) {
  const options = row.table.options;
  if (typeof options.enableMultiRowSelection === "function") return options.enableMultiRowSelection(row);
  return options.enableMultiRowSelection ?? true;
}
function row_getToggleSelectedHandler(row) {
  const canSelect = row_getCanSelect(row);
  return (e6) => {
    if (!canSelect) return;
    row_toggleSelected(row, e6.target.checked);
  };
}
var mutateRowIsSelected = (selectedRowIds, rowId, value, includeChildren, table) => {
  const row = table.getRow(rowId, true);
  if (value) {
    if (!row_getCanMultiSelect(row)) Object.keys(selectedRowIds).forEach((key) => delete selectedRowIds[key]);
    if (row_getCanSelect(row)) selectedRowIds[rowId] = true;
  } else delete selectedRowIds[rowId];
  if (includeChildren && row.subRows.length && row_getCanSelectSubRows(row)) row.subRows.forEach((r6) => mutateRowIsSelected(selectedRowIds, r6.id, value, includeChildren, table));
};
function selectRowsFn(rowModel) {
  const newSelectedFlatRows = [];
  const newSelectedRowsById = {};
  const recurseRows = (rows, depth = 0) => {
    const result = [];
    for (let i6 = 0; i6 < rows.length; i6++) {
      const row = rows[i6];
      const isSelected = isRowSelected(row);
      if (isSelected) {
        newSelectedFlatRows.push(row);
        newSelectedRowsById[row.id] = row;
      }
      if (row.subRows.length) {
        const newSubRows = recurseRows(row.subRows, depth + 1);
        if (isSelected) {
          const cloned = Object.create(Object.getPrototypeOf(row));
          Object.assign(cloned, row);
          cloned.subRows = newSubRows;
          result.push(cloned);
        }
      } else if (isSelected) result.push(row);
    }
    return result;
  };
  return {
    rows: recurseRows(rowModel.rows),
    flatRows: newSelectedFlatRows,
    rowsById: newSelectedRowsById
  };
}
function isRowSelected(row) {
  var _row$table$atoms$rowS;
  return (((_row$table$atoms$rowS = row.table.atoms.rowSelection) === null || _row$table$atoms$rowS === void 0 ? void 0 : _row$table$atoms$rowS.get()) ?? {})[row.id] ?? false;
}
function isSubRowSelected(row) {
  if (!row.subRows.length) return false;
  let allChildrenSelected = true;
  let someSelected = false;
  row.subRows.forEach((subRow) => {
    if (someSelected && !allChildrenSelected) return;
    if (row_getCanSelect(subRow)) if (isRowSelected(subRow)) someSelected = true;
    else allChildrenSelected = false;
    if (subRow.subRows.length) {
      const subRowChildrenSelected = isSubRowSelected(subRow);
      if (subRowChildrenSelected === "all") someSelected = true;
      else if (subRowChildrenSelected === "some") {
        someSelected = true;
        allChildrenSelected = false;
      } else allChildrenSelected = false;
    }
  });
  return allChildrenSelected ? "all" : someSelected ? "some" : false;
}

// node_modules/@tanstack/table-core/dist/features/row-selection/rowSelectionFeature.js
var rowSelectionFeature = {
  getInitialState: (initialState) => {
    return {
      rowSelection: getDefaultRowSelectionState(),
      ...initialState
    };
  },
  getDefaultTableOptions: (table) => {
    return {
      onRowSelectionChange: makeStateUpdater("rowSelection", table),
      enableRowSelection: true,
      enableMultiRowSelection: true,
      enableSubRowSelection: true
    };
  },
  assignRowPrototype: (prototype, table) => {
    assignPrototypeAPIs("rowSelectionFeature", prototype, table, {
      row_toggleSelected: { fn: (row, value, opts) => row_toggleSelected(row, value, opts) },
      row_getIsSelected: { fn: (row) => row_getIsSelected(row) },
      row_getIsSomeSelected: { fn: (row) => row_getIsSomeSelected(row) },
      row_getIsAllSubRowsSelected: { fn: (row) => row_getIsAllSubRowsSelected(row) },
      row_getCanSelect: { fn: (row) => row_getCanSelect(row) },
      row_getCanSelectSubRows: { fn: (row) => row_getCanSelectSubRows(row) },
      row_getCanMultiSelect: { fn: (row) => row_getCanMultiSelect(row) },
      row_getToggleSelectedHandler: { fn: (row) => row_getToggleSelectedHandler(row) }
    });
  },
  constructTableAPIs: (table) => {
    assignTableAPIs("rowSelectionFeature", table, {
      table_setRowSelection: { fn: (updater) => table_setRowSelection(table, updater) },
      table_resetRowSelection: { fn: (defaultState) => table_resetRowSelection(table, defaultState) },
      table_toggleAllRowsSelected: { fn: (value) => table_toggleAllRowsSelected(table, value) },
      table_toggleAllPageRowsSelected: { fn: (value) => table_toggleAllPageRowsSelected(table, value) },
      table_getPreSelectedRowModel: { fn: () => table_getPreSelectedRowModel(table) },
      table_getSelectedRowModel: {
        fn: () => table_getSelectedRowModel(table),
        memoDeps: () => {
          var _table$atoms$rowSelec;
          return [(_table$atoms$rowSelec = table.atoms.rowSelection) === null || _table$atoms$rowSelec === void 0 ? void 0 : _table$atoms$rowSelec.get(), table.getCoreRowModel()];
        }
      },
      table_getFilteredSelectedRowModel: {
        fn: () => table_getFilteredSelectedRowModel(table),
        memoDeps: () => {
          var _table$atoms$rowSelec2;
          return [(_table$atoms$rowSelec2 = table.atoms.rowSelection) === null || _table$atoms$rowSelec2 === void 0 ? void 0 : _table$atoms$rowSelec2.get(), table.getFilteredRowModel()];
        }
      },
      table_getGroupedSelectedRowModel: {
        fn: () => table_getGroupedSelectedRowModel(table),
        memoDeps: () => {
          var _table$atoms$rowSelec3;
          return [(_table$atoms$rowSelec3 = table.atoms.rowSelection) === null || _table$atoms$rowSelec3 === void 0 ? void 0 : _table$atoms$rowSelec3.get(), table.getSortedRowModel()];
        }
      },
      table_getIsAllRowsSelected: { fn: () => table_getIsAllRowsSelected(table) },
      table_getIsAllPageRowsSelected: { fn: () => table_getIsAllPageRowsSelected(table) },
      table_getIsSomeRowsSelected: { fn: () => table_getIsSomeRowsSelected(table) },
      table_getIsSomePageRowsSelected: { fn: () => table_getIsSomePageRowsSelected(table) },
      table_getToggleAllRowsSelectedHandler: { fn: () => table_getToggleAllRowsSelectedHandler(table) },
      table_getToggleAllPageRowsSelectedHandler: { fn: () => table_getToggleAllPageRowsSelectedHandler(table) }
    });
  }
};

// node_modules/@tanstack/table-core/dist/features/row-sorting/rowSortingFeature.utils.js
function getDefaultSortingState() {
  return [];
}
function table_setSorting(table, updater) {
  var _table$options$onSort, _table$options;
  (_table$options$onSort = (_table$options = table.options).onSortingChange) === null || _table$options$onSort === void 0 || _table$options$onSort.call(_table$options, updater);
}
function table_resetSorting(table, defaultState) {
  table_setSorting(table, defaultState ? [] : cloneState(table.initialState.sorting ?? []));
}
function column_getAutoSortFn(column) {
  const sortFns = column.table._rowModelFns.sortFns;
  const firstRows = column.table.getFilteredRowModel().flatRows.slice(0, 10);
  let isString = false;
  for (let i6 = 0; i6 < firstRows.length; i6++) {
    const value = firstRows[i6].getValue(column.id);
    if (Object.prototype.toString.call(value) === "[object Date]") {
      if (sortFns === null || sortFns === void 0 ? void 0 : sortFns.datetime) return sortFns.datetime;
    }
    if (typeof value === "string") {
      isString = true;
      if (value.split(reSplitAlphaNumeric).length > 1) {
        if (sortFns === null || sortFns === void 0 ? void 0 : sortFns.alphanumeric) return sortFns.alphanumeric;
      }
    }
  }
  if (isString) return (sortFns === null || sortFns === void 0 ? void 0 : sortFns.text) ?? sortFn_basic;
  return sortFn_basic;
}
function column_getAutoSortDir(column) {
  const firstRow = column.table.getFilteredRowModel().flatRows[0];
  if (typeof (firstRow ? firstRow.getValue(column.id) : void 0) === "string") return "asc";
  return "desc";
}
function column_getSortFn(column) {
  const sortFns = column.table._rowModelFns.sortFns;
  return isFunction(column.columnDef.sortFn) ? column.columnDef.sortFn : column.columnDef.sortFn === "auto" ? column_getAutoSortFn(column) : (sortFns === null || sortFns === void 0 ? void 0 : sortFns[column.columnDef.sortFn]) ?? sortFn_basic;
}
function column_toggleSorting(column, desc, multi) {
  const nextSortingOrder = column_getNextSortingOrder(column);
  const hasManualValue = typeof desc !== "undefined";
  table_setSorting(column.table, (old) => {
    const existingSorting = old.find((d3) => d3.id === column.id);
    const existingIndex = old.findIndex((d3) => d3.id === column.id);
    let newSorting = [];
    let sortAction;
    const nextDesc = hasManualValue ? desc : nextSortingOrder === "desc";
    if (old.length && column_getCanMultiSort(column) && multi) if (existingSorting) sortAction = "toggle";
    else sortAction = "add";
    else if (old.length && existingIndex !== old.length - 1) sortAction = "replace";
    else if (existingSorting) sortAction = "toggle";
    else sortAction = "replace";
    if (sortAction === "toggle") {
      if (!hasManualValue) {
        if (!nextSortingOrder) sortAction = "remove";
      }
    }
    if (sortAction === "add") {
      newSorting = [...old, {
        id: column.id,
        desc: nextDesc
      }];
      newSorting.splice(0, newSorting.length - (column.table.options.maxMultiSortColCount ?? Number.MAX_SAFE_INTEGER));
    } else if (sortAction === "toggle") newSorting = old.map((d3) => {
      if (d3.id === column.id) return {
        ...d3,
        desc: nextDesc
      };
      return d3;
    });
    else if (sortAction === "remove") newSorting = old.filter((d3) => d3.id !== column.id);
    else newSorting = [{
      id: column.id,
      desc: nextDesc
    }];
    return newSorting;
  });
}
function column_getFirstSortDir(column) {
  return column.columnDef.sortDescFirst ?? column.table.options.sortDescFirst ?? column_getAutoSortDir(column) === "desc" ? "desc" : "asc";
}
function column_getNextSortingOrder(column, multi) {
  const firstSortDirection = column_getFirstSortDir(column);
  const isSorted = column_getIsSorted(column);
  if (!isSorted) return firstSortDirection;
  if (isSorted !== firstSortDirection && (column.table.options.enableSortingRemoval ?? true) && (multi ? column.table.options.enableMultiRemove ?? true : true)) return false;
  return isSorted === "desc" ? "asc" : "desc";
}
function column_getCanSort(column) {
  return (column.columnDef.enableSorting ?? true) && (column.table.options.enableSorting ?? true) && !!column.accessorFn;
}
function column_getCanMultiSort(column) {
  return column.columnDef.enableMultiSort ?? column.table.options.enableMultiSort ?? !!column.accessorFn;
}
function column_getIsSorted(column) {
  var _column$table$atoms$s;
  const columnSort = (_column$table$atoms$s = column.table.atoms.sorting) === null || _column$table$atoms$s === void 0 || (_column$table$atoms$s = _column$table$atoms$s.get()) === null || _column$table$atoms$s === void 0 ? void 0 : _column$table$atoms$s.find((d3) => d3.id === column.id);
  return !columnSort ? false : columnSort.desc ? "desc" : "asc";
}
function column_getSortIndex(column) {
  var _column$table$atoms$s2;
  return ((_column$table$atoms$s2 = column.table.atoms.sorting) === null || _column$table$atoms$s2 === void 0 || (_column$table$atoms$s2 = _column$table$atoms$s2.get()) === null || _column$table$atoms$s2 === void 0 ? void 0 : _column$table$atoms$s2.findIndex((d3) => d3.id === column.id)) ?? -1;
}
function column_clearSorting(column) {
  table_setSorting(column.table, (old) => old.length ? old.filter((d3) => d3.id !== column.id) : []);
}
function column_getToggleSortingHandler(column) {
  const canSort = column_getCanSort(column);
  return (e6) => {
    var _persist, _column$table$options, _column$table$options2;
    if (!canSort) return;
    (_persist = e6.persist) === null || _persist === void 0 || _persist.call(e6);
    column_toggleSorting(column, void 0, column_getCanMultiSort(column) ? (_column$table$options = (_column$table$options2 = column.table.options).isMultiSortEvent) === null || _column$table$options === void 0 ? void 0 : _column$table$options.call(_column$table$options2, e6) : false);
  };
}

// node_modules/@tanstack/table-core/dist/features/row-sorting/rowSortingFeature.js
var rowSortingFeature = {
  getInitialState(initialState) {
    return {
      sorting: getDefaultSortingState(),
      ...initialState
    };
  },
  getDefaultColumnDef() {
    return {
      sortFn: "auto",
      sortUndefined: 1
    };
  },
  getDefaultTableOptions(table) {
    return {
      onSortingChange: makeStateUpdater("sorting", table),
      isMultiSortEvent: (e6) => {
        return e6.shiftKey;
      }
    };
  },
  assignColumnPrototype(prototype, table) {
    assignPrototypeAPIs("rowSortingFeature", prototype, table, {
      column_getAutoSortFn: { fn: (column) => column_getAutoSortFn(column) },
      column_getAutoSortDir: { fn: (column) => column_getAutoSortDir(column) },
      column_getSortFn: { fn: (column) => column_getSortFn(column) },
      column_toggleSorting: { fn: (column, desc, multi) => column_toggleSorting(column, desc, multi) },
      column_getFirstSortDir: { fn: (column) => column_getFirstSortDir(column) },
      column_getNextSortingOrder: { fn: (column, multi) => column_getNextSortingOrder(column, multi) },
      column_getCanSort: { fn: (column) => column_getCanSort(column) },
      column_getCanMultiSort: { fn: (column) => column_getCanMultiSort(column) },
      column_getIsSorted: { fn: (column) => column_getIsSorted(column) },
      column_getSortIndex: { fn: (column) => column_getSortIndex(column) },
      column_clearSorting: { fn: (column) => column_clearSorting(column) },
      column_getToggleSortingHandler: { fn: (column) => column_getToggleSortingHandler(column) }
    });
  },
  constructTableAPIs(table) {
    assignTableAPIs("rowSortingFeature", table, {
      table_setSorting: { fn: (updater) => table_setSorting(table, updater) },
      table_resetSorting: { fn: (defaultState) => table_resetSorting(table, defaultState) }
    });
  }
};

// node_modules/@tanstack/lit-table/dist/TableController.js
var TableController = class {
  constructor(host) {
    this._table = null;
    this._notifier = 0;
    (this.host = host).addController(this);
  }
  /**
  * Returns the Lit-backed table instance for the current render pass.
  *
  * The first call constructs the table with Lit reactivity bindings and
  * subscribes the host to table state/options changes. Later calls merge new
  * options into the same table instance and expose selected state through
  * `table.state`.
  *
  * @example
  * ```ts
  * const table = this.tableController.table(
  *   { features, columns, data },
  *   (state) => ({ sorting: state.sorting }),
  * )
  * ```
  */
  table(tableOptions, selector) {
    if (!this._table) {
      const mergedOptions = {
        ...tableOptions,
        features: {
          coreReactivityFeature: litReactivity(),
          ...tableOptions.features
        },
        mergeOptions: (defaultOptions, newOptions) => {
          return {
            ...defaultOptions,
            ...newOptions
          };
        }
      };
      this._table = constructTable(mergedOptions);
      this._setupSubscriptions();
    }
    this._table.setOptions((prev) => ({
      ...prev,
      ...tableOptions
    }));
    const tableInstance = this._table;
    const Subscribe = function Subscribe2(props) {
      const value = (props.source ?? tableInstance.store).get();
      const selectedState = props.selector !== void 0 ? props.selector(value) : value;
      if (typeof props.children === "function") return props.children(selectedState);
      return props.children;
    };
    return {
      ...this._table,
      Subscribe,
      FlexRender,
      get state() {
        return (selector === null || selector === void 0 ? void 0 : selector(tableInstance.store.state)) ?? tableInstance.store.state;
      }
    };
  }
  _setupSubscriptions() {
    if (this._table && !this._storeSubscription) {
      this._storeSubscription = this._table.store.subscribe(() => {
        this._notifier++;
        this.host.requestUpdate();
      });
      this._optionsSubscription = this._table.optionsStore.subscribe(() => {
        this._notifier++;
        this.host.requestUpdate();
      });
    }
  }
  hostConnected() {
    this._setupSubscriptions();
  }
  hostDisconnected() {
    var _this$_storeSubscript, _this$_optionsSubscri;
    (_this$_storeSubscript = this._storeSubscription) === null || _this$_storeSubscript === void 0 || _this$_storeSubscript.unsubscribe();
    this._storeSubscription = void 0;
    (_this$_optionsSubscri = this._optionsSubscription) === null || _this$_optionsSubscri === void 0 || _this$_optionsSubscri.unsubscribe();
    this._optionsSubscription = void 0;
  }
};

// web/components/visual-menu-icons.ts
function visualMenuIcon(name) {
  switch (name) {
    case "focus":
      return iconSvg(w`<path d="M3 7V5a2 2 0 0 1 2-2h2"></path><path d="M17 3h2a2 2 0 0 1 2 2v2"></path><path d="M21 17v2a2 2 0 0 1-2 2h-2"></path><path d="M7 21H5a2 2 0 0 1-2-2v-2"></path>`);
    case "show-data":
      return iconSvg(w`<path d="M3 5h18v14H3z"></path><path d="M3 10h18"></path><path d="M8 5v14"></path>`);
    case "copy-data":
      return iconSvg(w`<rect x="8" y="8" width="12" height="12" rx="2"></rect><path d="M16 8V6a2 2 0 0 0-2-2H6a2 2 0 0 0-2 2v8a2 2 0 0 0 2 2h2"></path>`);
    case "export-csv":
      return iconSvg(w`<path d="M12 3v12"></path><path d="m7 10 5 5 5-5"></path><path d="M5 21h14"></path>`);
    case "clear-selection":
      return iconSvg(w`<circle cx="12" cy="12" r="9"></circle><path d="m15 9-6 6"></path><path d="m9 9 6 6"></path>`);
  }
}
function iconSvg(content) {
  return w`<svg viewBox="0 0 24 24" aria-hidden="true">${content}</svg>`;
}

// web/components/table/format.ts
function formatCell(value, column) {
  if (value === null || value === void 0 || value === "") return "-";
  if ((column.key === "revenue" || column.measure === "revenue") && Number.isFinite(Number(value))) {
    return `R$ ${Number(value).toLocaleString(void 0, { maximumFractionDigits: 2 })}`;
  }
  if (column.key === "review_score" && Number.isFinite(Number(value))) {
    return Number(value).toFixed(2);
  }
  if (column.key === "delivery_days" && Number.isFinite(Number(value))) {
    return `${Number(value)}d`;
  }
  if (Number.isFinite(Number(value)) && column.align === "right") {
    return Number(value).toLocaleString(void 0, { maximumFractionDigits: 2 });
  }
  return String(value);
}
function defaultDirection(column) {
  return ["revenue", "review_score", "delivery_days", "purchase_date"].includes(column.key) || column.role === "measure" ? "desc" : "asc";
}
function rowKey(row, fallback) {
  const id = row.order_id;
  if (typeof id === "string" && id) return id;
  const rowID = row.__rowKey;
  if (typeof rowID === "string" && rowID) return rowID;
  return String(fallback);
}

// web/components/table/types.ts
var blockIDs = ["a", "b", "c"];
var defaultChunkSize = 200;
var defaultRowHeight = 34;
var defaultSort = { key: "purchase_date", direction: "desc" };

// web/components/table/block-source.ts
function emptyBlocks() {
  return {
    a: { start: 0, requestSeq: 0, resetVersion: 0, sort: defaultSort, rows: [] },
    b: { start: defaultChunkSize, requestSeq: 0, resetVersion: 0, sort: defaultSort, rows: [] },
    c: { start: defaultChunkSize * 2, requestSeq: 0, resetVersion: 0, sort: defaultSort, rows: [] }
  };
}
var emptyTable = {
  version: 2,
  kind: "data_table",
  title: "Orders",
  columns: [],
  totalRows: 0,
  availableRows: 0,
  isCapped: false,
  rowCap: 1e4,
  chunkSize: defaultChunkSize,
  rowHeight: defaultRowHeight,
  resetVersion: 0,
  sort: defaultSort,
  blocks: emptyBlocks(),
  loadingBlock: "",
  error: ""
};
var tableConverter = {
  fromAttribute(value) {
    if (!value) return emptyTable;
    try {
      return normalizeTable(JSON.parse(value));
    } catch {
      return { ...emptyTable, error: "Could not parse table signal." };
    }
  },
  toAttribute(value) {
    return JSON.stringify(value ?? emptyTable);
  }
};
function normalizeTable(value) {
  const chunkSize = positiveNumber(value.chunkSize, defaultChunkSize);
  return {
    ...emptyTable,
    ...value,
    version: 2,
    kind: value.kind === "matrix_table" || value.kind === "pivot_table" ? value.kind : "data_table",
    totalRows: positiveNumber(value.totalRows, 0),
    availableRows: positiveNumber(value.availableRows, positiveNumber(value.totalRows, 0)),
    rowCap: positiveNumber(value.rowCap, 1e4),
    chunkSize,
    rowHeight: positiveNumber(value.rowHeight, defaultRowHeight),
    resetVersion: positiveNumber(value.resetVersion, 0),
    sort: value.sort?.key ? value.sort : defaultSort,
    columns: Array.isArray(value.columns) ? value.columns : [],
    blocks: {
      a: normalizeBlock(value.blocks?.a, 0),
      b: normalizeBlock(value.blocks?.b, chunkSize),
      c: normalizeBlock(value.blocks?.c, chunkSize * 2)
    },
    loadingBlock: value.loadingBlock ?? "",
    error: value.error ?? ""
  };
}
function normalizeBlock(block, fallbackStart) {
  return {
    start: positiveNumber(block?.start, fallbackStart),
    requestSeq: positiveNumber(block?.requestSeq, 0),
    resetVersion: positiveNumber(block?.resetVersion, 0),
    sort: block?.sort?.key ? block.sort : defaultSort,
    rows: Array.isArray(block?.rows) ? block.rows : []
  };
}
function positiveNumber(value, fallback) {
  const next = Number(value);
  return Number.isFinite(next) && next >= 0 ? next : fallback;
}
function sameSort(a3, b3) {
  return a3.key === b3.key && a3.direction === b3.direction;
}
function sortedBlockRows(blocks, availableRows) {
  return blockIDs.map((id) => blocks[id]).sort((a3, b3) => a3.start - b3.start).flatMap((block) => block.rows.map((row, offset) => ({ row, index: block.start + offset }))).filter((item) => item.index < availableRows);
}

// web/components/data-table.ts
var tableFeaturesConfig = tableFeatures({
  columnPinningFeature,
  columnResizingFeature,
  columnSizingFeature,
  columnVisibilityFeature,
  rowSelectionFeature,
  rowSortingFeature
});
function defaultColumnSize(column) {
  const widths = {
    order_id: 240,
    purchase_date: 126,
    status: 128,
    state: 78,
    category: 210,
    revenue: 130,
    review_score: 104,
    delivery_days: 108
  };
  if (widths[column.key]) return widths[column.key];
  if (column.align === "right") return 120;
  return 140;
}
function applyUpdater(updater, current) {
  return typeof updater === "function" ? updater(current) : updater;
}
var DataTable = class extends i4 {
  constructor() {
    super(...arguments);
    this.tableId = "orders";
    this.table = emptyTable;
    this.selectedRowId = "";
    this.selectedCellKey = "";
    this.viewportTop = 0;
    this.viewportHeight = 0;
    this.columnVisibility = {};
    this.columnSizing = {};
    this.rowSelection = {};
    this.lastResetVersion = -1;
    this.shouldResetScroll = false;
    this.requestSeq = 0;
    this.scrollFrame = 0;
    this.jumpTimer = 0;
    this.pendingJumpStart = 0;
    this.expectedBlocks = /* @__PURE__ */ new Map();
    this.latestAcceptedSeq = /* @__PURE__ */ new Map();
    this.blockCache = emptyBlocks();
    this.scrollElementRef = e5();
    this.tableController = new TableController(this);
    this.handleOutsidePointerDown = (event) => {
      const details = this.renderRoot.querySelector(".visual-options");
      if (!details?.open) return;
      if (!event.composedPath().includes(details)) details.removeAttribute("open");
    };
    this.handleDocumentKeyDown = (event) => {
      if (event.key !== "Escape") return;
      this.renderRoot.querySelector(".visual-options")?.removeAttribute("open");
    };
  }
  static {
    this.properties = {
      tableId: { attribute: "table-id" },
      table: { attribute: "table", converter: tableConverter },
      selectedRowId: { state: true },
      selectedCellKey: { state: true },
      viewportTop: { state: true },
      viewportHeight: { state: true },
      columnVisibility: { state: true },
      columnSizing: { state: true },
      rowSelection: { state: true }
    };
  }
  static {
    this.styles = i`
    :host {
      display: block;
      height: 100%;
      min-height: 0;
      color: var(--fgColor-default);
      font-family: system-ui, -apple-system, BlinkMacSystemFont, 'Segoe UI', sans-serif;
    }

    .shell {
      display: flex;
      flex-direction: column;
      height: 100%;
      min-height: 0;
      background: var(--report-chart-surface, var(--card-bgColor, var(--bgColor-default)));
    }

    .toolbar {
      display: flex;
      align-items: center;
      justify-content: space-between;
      gap: 8px;
      min-height: 34px;
      border-bottom: 1px solid var(--borderColor-default);
      background: var(--report-chart-surface, var(--card-bgColor, var(--bgColor-default)));
      padding: 6px 8px 5px 10px;
    }

    h2 {
      min-width: 0;
      margin: 0;
      overflow: hidden;
      text-overflow: ellipsis;
      white-space: nowrap;
      font-size: 0.8rem;
      font-weight: 850;
      letter-spacing: 0;
      line-height: 1.1;
    }

    .visual-options {
      position: relative;
      flex: 0 0 auto;
    }

    .visual-options summary {
      display: grid;
      width: 24px;
      height: 24px;
      place-items: center;
      border: 1px solid transparent;
      border-radius: 4px;
      background: transparent;
      color: var(--fgColor-muted);
      cursor: pointer;
      font-size: 1rem;
      font-weight: 900;
      line-height: 1;
      list-style: none;
    }

    .visual-options summary::-webkit-details-marker {
      display: none;
    }

    .visual-options summary:hover,
    .visual-options summary:focus-visible,
    .visual-options[open] summary {
      border-color: var(--borderColor-default);
      background: var(--bgColor-muted);
      color: var(--fgColor-default);
      outline: 0;
    }

    .menu {
      position: absolute;
      top: calc(100% + 4px);
      right: 0;
      z-index: 30;
      display: grid;
      width: 176px;
      border: 1px solid var(--borderColor-default);
      border-radius: 6px;
      background: var(--overlay-bgColor, var(--bgColor-default));
      box-shadow: var(--shadow-floating-small, 0 8px 24px rgb(0 0 0 / 18%));
      padding: 4px;
    }

    .menu button {
      display: flex;
      align-items: center;
      gap: 8px;
      min-height: 27px;
      border: 0;
      border-radius: 4px;
      background: transparent;
      color: var(--fgColor-default);
      cursor: pointer;
      padding: 0 8px;
      font: inherit;
      font-size: 0.68rem;
      font-weight: 750;
      text-align: left;
    }

    .menu svg {
      flex: 0 0 auto;
      width: 14px;
      height: 14px;
      fill: none;
      stroke: currentColor;
      stroke-linecap: round;
      stroke-linejoin: round;
      stroke-width: 2;
    }

    .menu button:hover,
    .menu button:focus-visible {
      background: var(--bgColor-muted);
      outline: 0;
    }

    .menu button:disabled {
      cursor: default;
      opacity: 0.48;
    }

    .menu button:disabled:hover {
      background: transparent;
    }

    .menu-divider {
      height: 1px;
      margin: 4px 2px;
      background: var(--borderColor-muted);
    }

    .column-menu {
      display: grid;
      gap: 3px;
      padding: 2px;
    }

    .column-menu > span {
      padding: 2px 6px;
      color: var(--fgColor-muted);
      font-size: 0.62rem;
      font-weight: 900;
      text-transform: uppercase;
    }

    .column-menu label {
      display: flex;
      align-items: center;
      gap: 7px;
      min-height: 24px;
      border-radius: 4px;
      cursor: pointer;
      padding: 0 6px;
      font-size: 0.67rem;
      font-weight: 750;
    }

    .column-menu label:hover {
      background: var(--bgColor-muted);
    }

    .column-menu input {
      accent-color: var(--fgColor-accent);
    }

    .error {
      border-bottom: 1px solid var(--borderColor-danger-emphasis);
      background: var(--bgColor-danger-muted);
      color: var(--fgColor-danger);
      padding: 9px 12px;
      font-size: 0.82rem;
      font-weight: 850;
    }

    .head,
    .group-head,
    .row {
      display: grid;
      grid-template-columns: var(--ld-table-columns);
      min-width: 1080px;
    }

    .group-head {
      border-bottom: 1px solid var(--borderColor-default);
      background: color-mix(in srgb, var(--bgColor-muted), var(--report-chart-surface, var(--bgColor-default)) 34%);
      color: var(--fgColor-muted);
    }

    .group-cell {
      display: flex;
      align-items: center;
      min-width: 0;
      min-height: 26px;
      overflow: hidden;
      border-right: 1px solid var(--borderColor-default);
      padding: 0 9px;
      text-overflow: ellipsis;
      white-space: nowrap;
      font-size: 0.64rem;
      font-weight: 900;
      letter-spacing: 0;
      text-transform: uppercase;
    }

    .group-cell.measure-group {
      justify-content: center;
      color: var(--fgColor-default);
    }

    .group-cell:last-child {
      border-right: 0;
    }

    .head {
      position: relative;
      z-index: 1;
      border-bottom: 1px solid var(--borderColor-emphasis);
      background: var(--bgColor-muted);
      color: var(--fgColor-muted);
      box-shadow: inset 0 -1px 0 var(--borderColor-emphasis);
    }

    .header-cell,
    .cell {
      min-width: 0;
      overflow: hidden;
      text-overflow: ellipsis;
      white-space: nowrap;
    }

    .header-cell {
      position: relative;
      border-right: 1px solid var(--borderColor-default);
    }

    .header-cell.row-header,
    .cell.row-header,
    .group-cell.row-header {
      position: sticky;
      left: 0;
      z-index: 4;
      background: var(--report-chart-surface, var(--card-bgColor, var(--bgColor-default)));
      box-shadow: 1px 0 0 var(--borderColor-default);
    }

    .head .header-cell.row-header {
      background: var(--bgColor-muted);
    }

    .group-head .group-cell.row-header {
      background: color-mix(in srgb, var(--bgColor-muted), var(--report-chart-surface, var(--bgColor-default)) 34%);
    }

    .header-cell:last-child {
      border-right: 0;
    }

    button.header-button {
      display: flex;
      align-items: center;
      justify-content: space-between;
      gap: 8px;
      width: 100%;
      min-height: 34px;
      border: 0;
      border-bottom: 2px solid transparent;
      background: transparent;
      color: inherit;
      cursor: pointer;
      padding: 0 9px;
      font: inherit;
      font-size: 0.7rem;
      font-weight: 900;
      letter-spacing: 0;
      text-align: left;
      text-transform: uppercase;
    }

    button.header-button:hover,
    button.header-button:focus-visible,
    .sorted button.header-button {
      background: color-mix(in srgb, var(--fgColor-accent), transparent 92%);
      color: var(--fgColor-default);
      outline: 0;
    }

    .sorted button.header-button {
      border-bottom-color: var(--fgColor-accent);
    }

    .sort {
      display: inline-grid;
      min-width: 18px;
      place-items: center;
      color: var(--fgColor-accent);
      font-size: 0.82rem;
      opacity: 0;
    }

    .sorted .sort {
      opacity: 1;
    }

    .column-resizer {
      position: absolute;
      inset-block: 5px;
      right: -3px;
      z-index: 3;
      width: 6px;
      cursor: col-resize;
    }

    .column-resizer::after {
      content: '';
      position: absolute;
      inset-block: 3px;
      left: 2px;
      width: 2px;
      border-radius: 999px;
      background: transparent;
    }

    .header-cell:hover .column-resizer::after,
    .column-resizer.resizing::after {
      background: var(--fgColor-accent);
    }

    .viewport {
      position: relative;
      flex: 1 1 auto;
      overflow: auto;
      min-height: 0;
      background: var(--report-chart-surface, var(--card-bgColor, var(--bgColor-default)));
      scrollbar-gutter: stable;
    }

    .table-plane {
      position: relative;
      min-width: 1080px;
    }

    .sticky-header {
      position: sticky;
      top: 0;
      z-index: 8;
      min-width: 1080px;
      background: var(--bgColor-muted);
    }

    .canvas {
      position: relative;
      min-width: 1080px;
    }

    .row {
      position: absolute;
      inset-inline: 0;
      height: var(--ld-row-height, 34px);
      border-bottom: 1px solid var(--borderColor-muted);
      background: var(--report-chart-surface, var(--card-bgColor, var(--bgColor-default)));
      color: var(--fgColor-default);
    }

    .row:nth-child(even) {
      background: color-mix(in srgb, var(--report-table-stripe, var(--bgColor-muted)), var(--report-chart-surface, var(--bgColor-default)) 45%);
    }

    .row:hover {
      background: color-mix(in srgb, var(--fgColor-accent), transparent 91%);
    }

    .row.selected {
      background: color-mix(in srgb, var(--fgColor-accent), transparent 86%);
      box-shadow: inset 3px 0 0 var(--fgColor-accent);
    }

    .row.skeleton-row {
      pointer-events: none;
    }

    .row.skeleton-row:hover {
      background: var(--report-chart-surface, var(--card-bgColor, var(--bgColor-default)));
    }

    .cell {
      display: flex;
      align-items: center;
      min-width: 0;
      border: 0;
      border-right: 1px solid var(--borderColor-muted);
      background: transparent;
      color: inherit;
      cursor: default;
      font: inherit;
      padding: 0 9px;
      font-size: 0.77rem;
      font-weight: 600;
      text-align: left;
    }

    .cell:last-child {
      border-right: 0;
    }

    .cell.active {
      outline: 2px solid var(--fgColor-accent);
      outline-offset: -2px;
      background: color-mix(in srgb, var(--fgColor-accent), transparent 88%);
    }

    .skeleton-cell {
      cursor: default;
    }

    .skeleton-line {
      display: block;
      width: min(76%, 140px);
      height: 9px;
      overflow: hidden;
      border-radius: 999px;
      background: linear-gradient(
        90deg,
        var(--bgColor-muted) 0%,
        color-mix(in srgb, var(--fgColor-muted), transparent 82%) 45%,
        var(--bgColor-muted) 90%
      );
      background-size: 220% 100%;
      animation: shimmer 1.15s ease-in-out infinite;
      opacity: 0.78;
    }

    .skeleton-cell:nth-child(2n) .skeleton-line {
      width: min(58%, 120px);
    }

    .right {
      justify-content: end;
      font-variant-numeric: tabular-nums;
    }

    .empty {
      display: grid;
      min-height: 240px;
      place-items: center;
      color: var(--fgColor-muted);
      font-size: 0.9rem;
      font-weight: 850;
    }

    .loading {
      position: absolute;
      inset-inline: 0;
      top: 0;
      z-index: 2;
      height: 3px;
      overflow: hidden;
      background: var(--bgColor-accent-muted);
    }

    .loading::after {
      content: '';
      display: block;
      width: 34%;
      height: 100%;
      background: var(--fgColor-accent);
      animation: load 900ms ease-in-out infinite;
    }

    .footer {
      display: flex;
      align-items: center;
      justify-content: space-between;
      gap: 10px;
      min-height: 34px;
      border-top: 1px solid var(--borderColor-default);
      background: var(--report-panel-subtle, var(--bgColor-muted));
      padding: 6px 10px;
      color: var(--fgColor-muted);
      font-size: 0.72rem;
      font-weight: 750;
    }

    .footer strong {
      color: var(--fgColor-default);
      font-weight: 850;
    }

    @keyframes load {
      0% { transform: translateX(-100%); }
      100% { transform: translateX(310%); }
    }

    @keyframes shimmer {
      0% { background-position: 120% 0; }
      100% { background-position: -120% 0; }
    }

    @media (max-width: 760px) {
      .shell {
        min-height: 360px;
      }

      .toolbar,
      .footer {
        align-items: stretch;
        flex-direction: column;
      }
    }
  `;
  }
  connectedCallback() {
    super.connectedCallback();
    document.addEventListener("pointerdown", this.handleOutsidePointerDown);
    document.addEventListener("keydown", this.handleDocumentKeyDown);
  }
  firstUpdated() {
    const viewport = this.scrollElementRef.value;
    if (!viewport) return;
    this.viewportHeight = viewport.clientHeight;
    this.viewportLeft = viewport.scrollLeft;
    this.resizeObserver = new ResizeObserver(() => {
      this.viewportHeight = viewport.clientHeight;
      this.viewportLeft = viewport.scrollLeft;
      this.scheduleEnsureBlocksForScroll();
    });
    this.resizeObserver.observe(viewport);
    this.scheduleEnsureBlocksForScroll();
  }
  disconnectedCallback() {
    document.removeEventListener("pointerdown", this.handleOutsidePointerDown);
    document.removeEventListener("keydown", this.handleDocumentKeyDown);
    this.resizeObserver?.disconnect();
    if (this.scrollFrame) cancelAnimationFrame(this.scrollFrame);
    this.clearJumpTimer();
    super.disconnectedCallback();
  }
  willUpdate() {
    if (this.lastResetVersion !== this.table.resetVersion) {
      this.lastResetVersion = this.table.resetVersion;
      this.blockCache = emptyBlocks();
      this.shouldResetScroll = true;
      this.expectedBlocks.clear();
      this.latestAcceptedSeq.clear();
      this.clearJumpTimer();
      this.selectedRowId = "";
      this.selectedCellKey = "";
    }
    this.mergeIncomingBlocks();
    if (this.selectedRowId && !this.loadedRows.some((item) => rowKey(item.row, item.index) === this.selectedRowId)) {
      this.selectedRowId = "";
      this.selectedCellKey = "";
    }
  }
  updated() {
    if (this.shouldResetScroll) {
      this.shouldResetScroll = false;
      queueMicrotask(() => {
        const viewport = this.scrollElementRef.value;
        if (!viewport) return;
        viewport.scrollTop = 0;
        this.viewportTop = 0;
        this.viewportLeft = viewport.scrollLeft;
        this.viewportHeight = viewport.clientHeight;
        this.scheduleEnsureBlocksForScroll();
      });
    }
  }
  get columns() {
    return Array.isArray(this.table?.columns) ? this.table.columns : [];
  }
  get loadedRows() {
    return sortedBlockRows(this.blocks, this.availableRows);
  }
  get visibleRows() {
    if (this.availableRows <= 0) return [];
    const rowMap = new Map(this.loadedRows.map((item) => [item.index, item.row]));
    const first = Math.max(0, Math.floor(this.viewportTop / this.rowHeight) - 2);
    const visibleCount = Math.max(1, Math.ceil((this.viewportHeight || this.rowHeight) / this.rowHeight) + 4);
    const last = Math.min(this.availableRows, first + visibleCount);
    const rows = [];
    for (let index = first; index < last; index++) {
      const row = rowMap.get(index);
      rows.push(row ? { kind: "row", row, index } : { kind: "skeleton", index });
    }
    return rows;
  }
  get visibleLoading() {
    return this.visibleRows.some((row) => row.kind === "skeleton") || this.expectedBlocks.size > 0;
  }
  get availableRows() {
    return Math.max(0, this.table.availableRows ?? 0);
  }
  get blocks() {
    return this.blockCache;
  }
  get chunkSize() {
    return Math.max(1, this.table.chunkSize || defaultChunkSize);
  }
  get rowHeight() {
    return Math.max(1, this.table.rowHeight || defaultRowHeight);
  }
  get gridTemplate() {
    const widths = {
      __select: 34,
      order_id: 240,
      purchase_date: 126,
      status: 128,
      state: 78,
      category: 210,
      revenue: 130,
      review_score: 104,
      delivery_days: 108
    };
    return this.visibleColumnSizes.map(({ key, size }) => `${Math.max(44, size || widths[key] || 130)}px`).join(" ");
  }
  get tanstackRows() {
    return this.loadedRows.map(({ row, index }) => ({
      ...row,
      __absoluteIndex: index,
      __rowKey: rowKey(row, index)
    }));
  }
  get visibleColumnSizes() {
    const base = this.columnsForTanStack();
    return base.filter((column) => this.columnVisibility[column.key] !== false).map((column) => ({ key: column.key, size: this.columnSizing[column.key] ?? defaultColumnSize(column) }));
  }
  columnsForTanStack() {
    return this.columns;
  }
  groupHeaderSegments(columns) {
    if (!columns.some((column) => column.group)) return [];
    const segments = [];
    for (const column of columns) {
      const rowHeader = column.role === "row_header";
      const label = rowHeader ? "" : column.group || "";
      const previous = segments[segments.length - 1];
      if (previous && previous.label === label && previous.rowHeader === rowHeader) {
        previous.span++;
        continue;
      }
      segments.push({ label, span: 1, rowHeader });
    }
    return segments;
  }
  tanstackColumnDefs() {
    return this.columnsForTanStack().map((column) => ({
      id: column.key,
      accessorKey: column.key,
      header: column.label,
      cell: (info) => formatCell(info.getValue(), column),
      size: defaultColumnSize(column),
      minSize: column.key === "order_id" || column.key === "category" ? 160 : 64,
      enableResizing: true,
      meta: { align: column.align }
    }));
  }
  tanstackTable() {
    const firstColumn = this.columns[0]?.key;
    const sorting = this.table.sort?.key ? [{ id: this.table.sort.key, desc: this.table.sort.direction === "desc" }] : [];
    return this.tableController.table(
      {
        features: tableFeaturesConfig,
        columns: this.tanstackColumnDefs(),
        data: this.tanstackRows,
        getRowId: (row) => row.__rowKey,
        manualSorting: true,
        manualFiltering: true,
        manualPagination: true,
        enableRowSelection: true,
        enableMultiRowSelection: false,
        columnResizeMode: "onEnd",
        renderFallbackValue: "-",
        state: {
          sorting,
          columnVisibility: this.columnVisibility,
          columnSizing: this.columnSizing,
          columnPinning: { left: firstColumn ? [firstColumn] : [], right: [] },
          rowSelection: this.rowSelection
        },
        onColumnVisibilityChange: (updater) => {
          this.columnVisibility = applyUpdater(updater, this.columnVisibility);
        },
        onColumnSizingChange: (updater) => {
          this.columnSizing = applyUpdater(updater, this.columnSizing);
        },
        onRowSelectionChange: (updater) => {
          this.rowSelection = applyUpdater(updater, this.rowSelection);
          this.selectedRowId = Object.keys(this.rowSelection).find((key) => this.rowSelection[key]) ?? "";
          if (!this.selectedRowId) this.selectedCellKey = "";
        }
      },
      (state) => ({
        columnVisibility: state.columnVisibility,
        columnSizing: state.columnSizing,
        rowSelection: state.rowSelection,
        sorting: state.sorting
      })
    );
  }
  handleScroll(event) {
    const target = event.currentTarget;
    this.viewportTop = target.scrollTop;
    this.viewportLeft = target.scrollLeft;
    this.viewportHeight = target.clientHeight;
    this.scheduleEnsureBlocksForScroll();
  }
  sortColumn(column) {
    const current = this.table?.sort ?? defaultSort;
    const direction = current.key === column.key ? current.direction === "asc" ? "desc" : "asc" : defaultDirection(column);
    this.emitBlock("all", 0, { key: column.key, direction }, this.table.resetVersion + 1);
  }
  selectCell(row, column, absoluteIndex) {
    const key = rowKey(row, absoluteIndex);
    this.selectedRowId = key;
    this.selectedCellKey = `${key}:${column.key}`;
  }
  render() {
    const tanstack = this.tanstackTable();
    const headers = tanstack.getHeaderGroups()[0]?.headers ?? [];
    const visibleColumns = tanstack.getVisibleLeafColumns();
    const rowsByKey = new Map(tanstack.getRowModel().rows.map((row) => [row.original.__rowKey, row]));
    const columns = visibleColumns.map((column) => this.columns.find((item) => item.key === column.id)).filter(Boolean);
    const groupHeaders = this.groupHeaderSegments(columns);
    const visibleRows = this.visibleRows;
    const totalHeight = this.availableRows * this.rowHeight;
    const rowRange = this.rowRangeText();
    const selectedText = this.selectedRowId ? "1 row selected" : "No selection";
    const loading = Boolean(this.table.loadingBlock) || this.visibleLoading;
    return b2`
      <section class="shell" style=${`--ld-table-columns:${this.gridTemplate};--ld-row-height:${this.rowHeight}px`}>
        <div class="toolbar">
          <div>
            <h2>${this.table?.title ?? "Orders"}</h2>
          </div>
          <details class="visual-options">
            <summary aria-label="Visual options" title="Visual options">⋮</summary>
            <div class="menu" role="menu">
              <button type="button" role="menuitem" @click=${() => this.runAction("focus")}>${visualMenuIcon("focus")}<span>Focus mode</span></button>
              <button type="button" role="menuitem" @click=${() => this.runAction("show-data")}>${visualMenuIcon("show-data")}<span>Show data</span></button>
              <button type="button" role="menuitem" @click=${() => this.runAction("copy-data")}>${visualMenuIcon("copy-data")}<span>Copy data</span></button>
              <button type="button" role="menuitem" @click=${() => this.runAction("export-csv")}>${visualMenuIcon("export-csv")}<span>Export CSV</span></button>
              <button type="button" role="menuitem" ?disabled=${!this.selectedRowId} @click=${() => this.runAction("clear-selection")}>${visualMenuIcon("clear-selection")}<span>Clear selection</span></button>
              <div class="menu-divider"></div>
              <div class="column-menu" @click=${(event) => event.stopPropagation()}>
                <span>Columns</span>
                ${tanstack.getAllLeafColumns().map((column) => b2`
                  <label>
                    <input
                      type="checkbox"
                      .checked=${column.getIsVisible()}
                      ?disabled=${!column.getCanHide()}
                      @change=${column.getToggleVisibilityHandler()}
                    />
                    ${column.columnDef.header}
                  </label>
                `)}
              </div>
            </div>
          </details>
        </div>
        ${this.table?.error ? b2`<div class="error">${this.table.error}</div>` : A}
        <div class="viewport" ${n5(this.scrollElementRef)} @scroll=${this.handleScroll} role="table" aria-label=${this.table?.title ?? "Orders"}>
          ${loading ? b2`<div class="loading" aria-hidden="true"></div>` : A}
          ${this.availableRows === 0 && !loading ? b2`<div class="empty">Waiting for table data</div>` : b2`
            <div class="table-plane">
              <div class="sticky-header">
                ${groupHeaders.length ? b2`
                  <div class="group-head" role="row">
                    ${groupHeaders.map((group) => b2`
                      <div
                        class=${`group-cell ${group.rowHeader ? "row-header" : "measure-group"}`}
                        role="columnheader"
                        style=${`grid-column:span ${group.span}`}
                      >
                        ${group.label}
                      </div>
                    `)}
                  </div>
                ` : A}
                <div class="head" role="row">
                  ${headers.map((header) => {
      const column = this.columns.find((item) => item.key === header.column.id);
      if (!column) return A;
      const sorted = this.table?.sort?.key === header.column.id;
      const sortMark = this.table?.sort?.direction === "asc" ? "\u2191" : "\u2193";
      return b2`
                      <div class=${`header-cell ${column.role === "row_header" ? "row-header" : ""} ${sorted ? "sorted" : ""}`} role="columnheader">
                        <button class="header-button" type="button" @click=${() => this.sortColumn(column)}>
                          <span>${FlexRender({ header })}</span>
                          <span class="sort">${sortMark}</span>
                        </button>
                        ${header.column.getCanResize?.() ? b2`
                          <span
                            class=${`column-resizer ${header.column.getIsResizing?.() ? "resizing" : ""}`}
                            @mousedown=${header.getResizeHandler?.()}
                            @touchstart=${header.getResizeHandler?.()}
                          ></span>
                        ` : A}
                      </div>
                    `;
    })}
                </div>
              </div>
              <div class="canvas" style=${`height:${totalHeight}px`}>
                ${visibleRows.map((slot) => {
      if (slot.kind === "skeleton") {
        return b2`
                      <div
                        class="row skeleton-row"
                        role="row"
                        aria-busy="true"
                        style=${`top:${slot.index * this.rowHeight}px`}
                      >
                        ${columns.map((column) => b2`
                          <span class=${`cell skeleton-cell ${column.role === "row_header" ? "row-header" : ""} ${column.align === "right" ? "right" : ""}`} role="cell">
                            <span class="skeleton-line"></span>
                          </span>
                        `)}
                      </div>
                    `;
      }
      const { row, index } = slot;
      const key = rowKey(row, index);
      const selected = key === this.selectedRowId;
      const tanstackRow = rowsByKey.get(key);
      const cells = tanstackRow?.getVisibleCells?.() ?? [];
      return b2`
                    <div
                      class=${`row ${selected ? "selected" : ""}`}
                      role="row"
                      aria-selected=${selected ? "true" : "false"}
                      style=${`top:${index * this.rowHeight}px`}
                      @click=${() => {
        this.selectedRowId = key;
        this.rowSelection = { [key]: true };
        this.selectedCellKey = "";
      }}
                    >
                      ${cells.map((cell) => {
        const column = this.columns.find((item) => item.key === cell.column.id);
        if (!column) return A;
        const cellKey = `${key}:${cell.column.id}`;
        return b2`
                          <button
                            class=${`cell ${column.align === "right" ? "right" : ""} ${column.role === "row_header" ? "row-header" : ""} ${cellKey === this.selectedCellKey ? "active" : ""}`}
                            role="cell"
                            title=${String(row[cell.column.id] ?? "")}
                            @click=${(event) => {
          event.stopPropagation();
          this.selectCell(row, column, index);
        }}
                          >
                            ${FlexRender({ cell })}
                          </button>
                        `;
      })}
                    </div>
                  `;
    })}
              </div>
            </div>
          `}
        </div>
        <div class="footer">
          <span><strong>${rowRange}</strong>${this.visibleLoading ? b2` · loading` : A}${this.table.isCapped ? b2` · browsing first ${this.table.rowCap.toLocaleString()}` : A}</span>
          <span>${selectedText}</span>
        </div>
      </section>
    `;
  }
  ensureBlocksForScroll() {
    if (this.availableRows <= 0) return;
    const currentStart = Math.floor(Math.floor(this.viewportTop / this.rowHeight) / this.chunkSize) * this.chunkSize;
    const desired = this.desiredStarts(currentStart);
    const desiredSet = new Set(desired);
    const loadedStarts = new Set(blockIDs.map((id) => this.blocks[id]?.start ?? -1));
    const expectedStarts = new Set([...this.expectedBlocks.values()].map((request) => request.start));
    const missingStarts = desired.filter((start) => !loadedStarts.has(start) && !expectedStarts.has(start));
    if (missingStarts.length > 1 || !loadedStarts.has(currentStart) && !expectedStarts.has(currentStart)) {
      this.scheduleJumpBlock(currentStart);
      return;
    }
    this.clearJumpTimer();
    const usedBlocks = /* @__PURE__ */ new Set();
    for (const start of missingStarts) {
      const block = this.reusableBlock(desiredSet, usedBlocks);
      if (!block) continue;
      usedBlocks.add(block);
      this.emitBlock(block, start, this.table.sort, this.table.resetVersion);
    }
  }
  scheduleEnsureBlocksForScroll() {
    if (this.scrollFrame) return;
    this.scrollFrame = requestAnimationFrame(() => {
      this.scrollFrame = 0;
      this.ensureBlocksForScroll();
    });
  }
  scheduleJumpBlock(start) {
    if (this.jumpTimer && this.pendingJumpStart === start) return;
    this.pendingJumpStart = start;
    this.requestUpdate();
    this.clearJumpTimer();
    this.jumpTimer = window.setTimeout(() => {
      this.jumpTimer = 0;
      this.emitBlock("all", this.pendingJumpStart, this.table.sort, this.table.resetVersion);
    }, 75);
  }
  clearJumpTimer() {
    if (!this.jumpTimer) return;
    clearTimeout(this.jumpTimer);
    this.jumpTimer = 0;
  }
  desiredStarts(currentStart) {
    const starts = currentStart <= 0 ? [0, this.chunkSize, this.chunkSize * 2] : [Math.max(0, currentStart - this.chunkSize), currentStart, currentStart + this.chunkSize];
    return starts.filter((start, index, all) => start < this.availableRows && all.indexOf(start) === index);
  }
  reusableBlock(desiredStarts, usedBlocks) {
    return blockIDs.find((id) => !usedBlocks.has(id) && !desiredStarts.has(this.blocks[id]?.start ?? -1)) ?? blockIDs.find((id) => !usedBlocks.has(id));
  }
  emitBlock(block, start, sort = this.table.sort, resetVersion = this.table.resetVersion) {
    const count = this.chunkSize;
    const requestSeq = ++this.requestSeq;
    if (block === "all") {
      this.expectedBlocks.clear();
      const starts = this.allBlockStarts(start);
      blockIDs.forEach((id, index) => {
        const expectedStart = starts[index];
        this.expectedBlocks.set(id, { start: expectedStart, requestSeq, resetVersion, sort });
      });
    } else {
      this.expectedBlocks.set(block, { start, requestSeq, resetVersion, sort });
    }
    this.requestUpdate();
    this.dispatchEvent(new CustomEvent("ld-table-window-change", {
      bubbles: true,
      composed: true,
      detail: {
        table: this.tableId || "orders",
        block,
        start,
        count,
        requestSeq,
        sort,
        resetVersion
      }
    }));
  }
  allBlockStarts(start) {
    const currentStart = Math.max(0, Math.floor(start / this.chunkSize) * this.chunkSize);
    if (currentStart <= 0) return [0, this.chunkSize, this.chunkSize * 2];
    return [Math.max(0, currentStart - this.chunkSize), currentStart, currentStart + this.chunkSize];
  }
  rowRangeText() {
    if (!this.table.totalRows || !this.availableRows) return "No rows";
    const firstIndex = Math.min(this.availableRows - 1, Math.max(0, Math.floor(this.viewportTop / this.rowHeight)));
    const visibleRows = Math.max(1, Math.ceil((this.viewportHeight || this.rowHeight) / this.rowHeight));
    const lastIndex = Math.min(this.availableRows, firstIndex + visibleRows);
    return `${(firstIndex + 1).toLocaleString()}-${lastIndex.toLocaleString()} of ${this.table.totalRows.toLocaleString()}`;
  }
  mergeIncomingBlocks() {
    const defaults = emptyBlocks();
    for (const id of blockIDs) {
      const incoming = this.table.blocks[id];
      if (!incoming) continue;
      if (!this.shouldAcceptBlock(id, incoming)) continue;
      const defaultBlock = defaults[id];
      const carriesRows = incoming.rows.length > 0;
      const carriesNonDefaultStart = incoming.start !== defaultBlock.start;
      const cacheIsEmpty = this.blockCache[id].rows.length === 0;
      if (carriesRows || carriesNonDefaultStart || cacheIsEmpty) {
        this.blockCache[id] = { ...incoming, rows: incoming.rows };
        if (incoming.requestSeq > 0) this.latestAcceptedSeq.set(id, incoming.requestSeq);
        const expected = this.expectedBlocks.get(id);
        if (expected && this.blockMatchesExpected(incoming, expected)) {
          this.expectedBlocks.delete(id);
        }
      }
    }
  }
  shouldAcceptBlock(id, incoming) {
    const expected = this.expectedBlocks.get(id);
    if (expected) return this.blockMatchesExpected(incoming, expected);
    if (incoming.requestSeq > 0) {
      const lastAcceptedSeq = this.latestAcceptedSeq.get(id) ?? 0;
      return incoming.requestSeq >= lastAcceptedSeq && incoming.resetVersion === this.table.resetVersion && sameSort(incoming.sort, this.table.sort);
    }
    return incoming.resetVersion === 0 || incoming.resetVersion === this.table.resetVersion && sameSort(incoming.sort, this.table.sort);
  }
  blockMatchesExpected(block, expected) {
    return block.start === expected.start && block.requestSeq === expected.requestSeq && block.resetVersion === expected.resetVersion && sameSort(block.sort, expected.sort);
  }
  runAction(action) {
    this.renderRoot.querySelector(".visual-options")?.removeAttribute("open");
    if (action === "clear-selection") {
      this.selectedRowId = "";
      this.selectedCellKey = "";
    }
    this.dispatchEvent(
      new CustomEvent("ld-visual-action", {
        bubbles: true,
        composed: true,
        detail: {
          action,
          visualType: "table",
          visualId: this.tableId || "orders",
          title: this.table?.title ?? "Orders",
          columns: this.columns,
          rows: this.exportRows(),
          selection: this.selectedRowId ? [this.selectedRowId] : [],
          table: {
            ...this.table ?? emptyTable,
            blocks: this.blocks,
            rows: this.exportRows(),
            columns: this.columns
          }
        }
      })
    );
  }
  exportRows() {
    return this.loadedRows.map(({ row }) => {
      const next = {};
      for (const column of this.columns) {
        next[column.key] = formatCell(row[column.key], column);
      }
      return next;
    });
  }
};
customElements.define("ld-data-table", DataTable);
/*! Bundled license information:

@lit/reactive-element/css-tag.js:
  (**
   * @license
   * Copyright 2019 Google LLC
   * SPDX-License-Identifier: BSD-3-Clause
   *)

@lit/reactive-element/reactive-element.js:
lit-html/lit-html.js:
lit-element/lit-element.js:
lit-html/directive.js:
lit-html/async-directive.js:
  (**
   * @license
   * Copyright 2017 Google LLC
   * SPDX-License-Identifier: BSD-3-Clause
   *)

lit-html/is-server.js:
  (**
   * @license
   * Copyright 2022 Google LLC
   * SPDX-License-Identifier: BSD-3-Clause
   *)

lit-html/directive-helpers.js:
lit-html/directives/ref.js:
  (**
   * @license
   * Copyright 2020 Google LLC
   * SPDX-License-Identifier: BSD-3-Clause
   *)
*/
