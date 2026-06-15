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

// web/components/data-table.ts
var blockIDs = ["a", "b", "c"];
var defaultChunkSize = 200;
var defaultRowHeight = 34;
var defaultSort = { key: "purchase_date", direction: "desc" };
var emptyTable = {
  version: 2,
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
function emptyBlocks() {
  return {
    a: { start: 0, rows: [] },
    b: { start: defaultChunkSize, rows: [] },
    c: { start: defaultChunkSize * 2, rows: [] }
  };
}
function normalizeTable(value) {
  const chunkSize = positiveNumber(value.chunkSize, defaultChunkSize);
  return {
    ...emptyTable,
    ...value,
    version: 2,
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
    rows: Array.isArray(block?.rows) ? block.rows : []
  };
}
function positiveNumber(value, fallback) {
  const next = Number(value);
  return Number.isFinite(next) && next >= 0 ? next : fallback;
}
function formatCell(value, column) {
  if (value === null || value === void 0 || value === "") return "-";
  if (column.key === "revenue" && Number.isFinite(Number(value))) {
    return `R$ ${Number(value).toLocaleString(void 0, { maximumFractionDigits: 2 })}`;
  }
  if (column.key === "review_score" && Number.isFinite(Number(value))) {
    return Number(value).toFixed(2);
  }
  if (column.key === "delivery_days" && Number.isFinite(Number(value))) {
    return `${Number(value)}d`;
  }
  return String(value);
}
function defaultDirection(column) {
  return ["revenue", "review_score", "delivery_days", "purchase_date"].includes(column.key) ? "desc" : "asc";
}
function rowKey(row, fallback) {
  const id = row.order_id;
  return typeof id === "string" && id ? id : String(fallback);
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
    this.lastResetVersion = -1;
    this.shouldResetScroll = false;
    this.pendingLoads = /* @__PURE__ */ new Set();
    this.blockCache = emptyBlocks();
    this.scrollElementRef = e5();
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
      viewportHeight: { state: true }
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

    .error {
      border-bottom: 1px solid var(--borderColor-danger-emphasis);
      background: var(--bgColor-danger-muted);
      color: var(--fgColor-danger);
      padding: 9px 12px;
      font-size: 0.82rem;
      font-weight: 850;
    }

    .head,
    .row {
      display: grid;
      grid-template-columns: var(--ld-table-columns);
      min-width: 1080px;
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
      border-right: 1px solid var(--borderColor-default);
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

    .viewport {
      position: relative;
      flex: 1 1 auto;
      overflow: auto;
      min-height: 0;
      background: var(--report-chart-surface, var(--card-bgColor, var(--bgColor-default)));
      scrollbar-gutter: stable;
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
    this.resizeObserver = new ResizeObserver(() => {
      this.viewportHeight = viewport.clientHeight;
      this.ensureBlocksForScroll();
    });
    this.resizeObserver.observe(viewport);
    this.ensureBlocksForScroll();
  }
  disconnectedCallback() {
    document.removeEventListener("pointerdown", this.handleOutsidePointerDown);
    document.removeEventListener("keydown", this.handleDocumentKeyDown);
    this.resizeObserver?.disconnect();
    super.disconnectedCallback();
  }
  willUpdate() {
    if (this.lastResetVersion !== this.table.resetVersion) {
      this.lastResetVersion = this.table.resetVersion;
      this.blockCache = emptyBlocks();
      this.shouldResetScroll = true;
      this.pendingLoads.clear();
      this.selectedRowId = "";
      this.selectedCellKey = "";
    }
    this.mergeIncomingBlocks();
    this.clearArrivedLoads();
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
        this.viewportHeight = viewport.clientHeight;
      });
    }
  }
  get columns() {
    return Array.isArray(this.table?.columns) ? this.table.columns : [];
  }
  get loadedRows() {
    return blockIDs.map((id) => this.blocks[id]).sort((a3, b3) => a3.start - b3.start).flatMap((block) => block.rows.map((row, offset) => ({ row, index: block.start + offset }))).filter((item) => item.index < this.availableRows);
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
      order_id: "minmax(210px,1.35fr)",
      purchase_date: "minmax(118px,.75fr)",
      status: "minmax(118px,.75fr)",
      state: "minmax(70px,.42fr)",
      category: "minmax(190px,1.1fr)",
      revenue: "minmax(120px,.72fr)",
      review_score: "minmax(96px,.55fr)",
      delivery_days: "minmax(96px,.55fr)"
    };
    return this.columns.map((column) => widths[column.key] ?? "minmax(120px,1fr)").join(" ");
  }
  handleScroll(event) {
    const target = event.currentTarget;
    this.viewportTop = target.scrollTop;
    this.viewportHeight = target.clientHeight;
    this.ensureBlocksForScroll();
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
    const columns = this.columns;
    const loadedRows = this.loadedRows;
    const totalHeight = this.availableRows * this.rowHeight;
    const rowRange = this.rowRangeText();
    const selectedText = this.selectedRowId ? "1 row selected" : "No selection";
    const loading = Boolean(this.table.loadingBlock);
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
            </div>
          </details>
        </div>
        ${this.table?.error ? b2`<div class="error">${this.table.error}</div>` : A}
        <div class="head" role="row">
          ${columns.map((column) => {
      const sorted = this.table?.sort?.key === column.key;
      const sortMark = this.table?.sort?.direction === "asc" ? "\u2191" : "\u2193";
      return b2`
              <div class=${`header-cell ${sorted ? "sorted" : ""}`} role="columnheader">
                <button class="header-button" type="button" @click=${() => this.sortColumn(column)}>
                  <span>${column.label}</span>
                  <span class="sort">${sortMark}</span>
                </button>
              </div>
            `;
    })}
        </div>
        <div class="viewport" ${n5(this.scrollElementRef)} @scroll=${this.handleScroll} role="table" aria-label=${this.table?.title ?? "Orders"}>
          ${loading ? b2`<div class="loading" aria-hidden="true"></div>` : A}
          ${this.availableRows === 0 && !loading ? b2`<div class="empty">Waiting for table data</div>` : b2`
            <div class="canvas" style=${`height:${totalHeight}px`}>
              ${loadedRows.map(({ row, index }) => {
      const key = rowKey(row, index);
      const selected = key === this.selectedRowId;
      return b2`
                  <div
                    class=${`row ${selected ? "selected" : ""}`}
                    role="row"
                    aria-selected=${selected ? "true" : "false"}
                    style=${`transform:translateY(${index * this.rowHeight}px)`}
                    @click=${() => {
        this.selectedRowId = key;
        this.selectedCellKey = "";
      }}
                  >
                    ${columns.map((column) => {
        const cellKey = `${key}:${column.key}`;
        return b2`
                        <button
                          class=${`cell ${column.align === "right" ? "right" : ""} ${cellKey === this.selectedCellKey ? "active" : ""}`}
                          role="cell"
                          title=${String(row[column.key] ?? "")}
                          @click=${(event) => {
          event.stopPropagation();
          this.selectCell(row, column, index);
        }}
                        >
                          ${formatCell(row[column.key], column)}
                        </button>
                      `;
      })}
                  </div>
                `;
    })}
            </div>
          `}
        </div>
        <div class="footer">
          <span><strong>${rowRange}</strong>${this.table.isCapped ? b2` · browsing first ${this.table.rowCap.toLocaleString()}` : A}</span>
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
    const pendingStarts = new Set([...this.pendingLoads].map((key) => Number(key.split(":")[1])));
    const usedBlocks = /* @__PURE__ */ new Set();
    for (const start of desired) {
      if (loadedStarts.has(start) || pendingStarts.has(start)) continue;
      const block = this.reusableBlock(desiredSet, usedBlocks);
      if (!block) continue;
      usedBlocks.add(block);
      this.emitBlock(block, start, this.table.sort, this.table.resetVersion);
    }
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
    const key = `${block}:${start}:${sort.key}:${sort.direction}:${resetVersion}`;
    if (this.pendingLoads.has(key)) return;
    this.pendingLoads.add(key);
    this.dispatchEvent(new CustomEvent("ld-table-window-change", {
      bubbles: true,
      composed: true,
      detail: {
        table: this.tableId || "orders",
        block,
        start,
        count,
        sort,
        resetVersion
      }
    }));
  }
  clearArrivedLoads() {
    for (const key of [...this.pendingLoads]) {
      const [block, start, sortKey, sortDirection, resetVersion] = key.split(":");
      if (block === "all") {
        if (this.table.resetVersion === Number(resetVersion) && this.table.sort.key === sortKey && this.table.sort.direction === sortDirection) {
          this.pendingLoads.delete(key);
        }
        continue;
      }
      const tableBlock = this.blocks[block];
      if (tableBlock?.start === Number(start) && this.table.sort.key === sortKey && this.table.sort.direction === sortDirection && this.table.resetVersion === Number(resetVersion)) {
        this.pendingLoads.delete(key);
      }
    }
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
      const defaultBlock = defaults[id];
      const carriesRows = incoming.rows.length > 0;
      const carriesNonDefaultStart = incoming.start !== defaultBlock.start;
      const cacheIsEmpty = this.blockCache[id].rows.length === 0;
      if (carriesRows || carriesNonDefaultStart || cacheIsEmpty) {
        this.blockCache[id] = { start: incoming.start, rows: incoming.rows };
      }
    }
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
