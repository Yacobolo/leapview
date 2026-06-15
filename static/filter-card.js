// node_modules/@lit/reactive-element/css-tag.js
var t = globalThis;
var e = t.ShadowRoot && (void 0 === t.ShadyCSS || t.ShadyCSS.nativeShadow) && "adoptedStyleSheets" in Document.prototype && "replace" in CSSStyleSheet.prototype;
var s = /* @__PURE__ */ Symbol();
var o = /* @__PURE__ */ new WeakMap();
var n = class {
  constructor(t3, e4, o5) {
    if (this._$cssResult$ = true, o5 !== s) throw Error("CSSResult is not constructable. Use `unsafeCSS` or `css` instead.");
    this.cssText = t3, this.t = e4;
  }
  get styleSheet() {
    let t3 = this.o;
    const s4 = this.t;
    if (e && void 0 === t3) {
      const e4 = void 0 !== s4 && 1 === s4.length;
      e4 && (t3 = o.get(s4)), void 0 === t3 && ((this.o = t3 = new CSSStyleSheet()).replaceSync(this.cssText), e4 && o.set(s4, t3));
    }
    return t3;
  }
  toString() {
    return this.cssText;
  }
};
var r = (t3) => new n("string" == typeof t3 ? t3 : t3 + "", void 0, s);
var i = (t3, ...e4) => {
  const o5 = 1 === t3.length ? t3[0] : e4.reduce((e5, s4, o6) => e5 + ((t4) => {
    if (true === t4._$cssResult$) return t4.cssText;
    if ("number" == typeof t4) return t4;
    throw Error("Value passed to 'css' function must be a 'css' function result: " + t4 + ". Use 'unsafeCSS' to pass non-literal values, but take care to ensure page security.");
  })(s4) + t3[o6 + 1], t3[0]);
  return new n(o5, t3, s);
};
var S = (s4, o5) => {
  if (e) s4.adoptedStyleSheets = o5.map((t3) => t3 instanceof CSSStyleSheet ? t3 : t3.styleSheet);
  else for (const e4 of o5) {
    const o6 = document.createElement("style"), n4 = t.litNonce;
    void 0 !== n4 && o6.setAttribute("nonce", n4), o6.textContent = e4.cssText, s4.appendChild(o6);
  }
};
var c = e ? (t3) => t3 : (t3) => t3 instanceof CSSStyleSheet ? ((t4) => {
  let e4 = "";
  for (const s4 of t4.cssRules) e4 += s4.cssText;
  return r(e4);
})(t3) : t3;

// node_modules/@lit/reactive-element/reactive-element.js
var { is: i2, defineProperty: e2, getOwnPropertyDescriptor: h, getOwnPropertyNames: r2, getOwnPropertySymbols: o2, getPrototypeOf: n2 } = Object;
var a = globalThis;
var c2 = a.trustedTypes;
var l = c2 ? c2.emptyScript : "";
var p = a.reactiveElementPolyfillSupport;
var d = (t3, s4) => t3;
var u = { toAttribute(t3, s4) {
  switch (s4) {
    case Boolean:
      t3 = t3 ? l : null;
      break;
    case Object:
    case Array:
      t3 = null == t3 ? t3 : JSON.stringify(t3);
  }
  return t3;
}, fromAttribute(t3, s4) {
  let i5 = t3;
  switch (s4) {
    case Boolean:
      i5 = null !== t3;
      break;
    case Number:
      i5 = null === t3 ? null : Number(t3);
      break;
    case Object:
    case Array:
      try {
        i5 = JSON.parse(t3);
      } catch (t4) {
        i5 = null;
      }
  }
  return i5;
} };
var f = (t3, s4) => !i2(t3, s4);
var b = { attribute: true, type: String, converter: u, reflect: false, useDefault: false, hasChanged: f };
Symbol.metadata ??= /* @__PURE__ */ Symbol("metadata"), a.litPropertyMetadata ??= /* @__PURE__ */ new WeakMap();
var y = class extends HTMLElement {
  static addInitializer(t3) {
    this._$Ei(), (this.l ??= []).push(t3);
  }
  static get observedAttributes() {
    return this.finalize(), this._$Eh && [...this._$Eh.keys()];
  }
  static createProperty(t3, s4 = b) {
    if (s4.state && (s4.attribute = false), this._$Ei(), this.prototype.hasOwnProperty(t3) && ((s4 = Object.create(s4)).wrapped = true), this.elementProperties.set(t3, s4), !s4.noAccessor) {
      const i5 = /* @__PURE__ */ Symbol(), h3 = this.getPropertyDescriptor(t3, i5, s4);
      void 0 !== h3 && e2(this.prototype, t3, h3);
    }
  }
  static getPropertyDescriptor(t3, s4, i5) {
    const { get: e4, set: r4 } = h(this.prototype, t3) ?? { get() {
      return this[s4];
    }, set(t4) {
      this[s4] = t4;
    } };
    return { get: e4, set(s5) {
      const h3 = e4?.call(this);
      r4?.call(this, s5), this.requestUpdate(t3, h3, i5);
    }, configurable: true, enumerable: true };
  }
  static getPropertyOptions(t3) {
    return this.elementProperties.get(t3) ?? b;
  }
  static _$Ei() {
    if (this.hasOwnProperty(d("elementProperties"))) return;
    const t3 = n2(this);
    t3.finalize(), void 0 !== t3.l && (this.l = [...t3.l]), this.elementProperties = new Map(t3.elementProperties);
  }
  static finalize() {
    if (this.hasOwnProperty(d("finalized"))) return;
    if (this.finalized = true, this._$Ei(), this.hasOwnProperty(d("properties"))) {
      const t4 = this.properties, s4 = [...r2(t4), ...o2(t4)];
      for (const i5 of s4) this.createProperty(i5, t4[i5]);
    }
    const t3 = this[Symbol.metadata];
    if (null !== t3) {
      const s4 = litPropertyMetadata.get(t3);
      if (void 0 !== s4) for (const [t4, i5] of s4) this.elementProperties.set(t4, i5);
    }
    this._$Eh = /* @__PURE__ */ new Map();
    for (const [t4, s4] of this.elementProperties) {
      const i5 = this._$Eu(t4, s4);
      void 0 !== i5 && this._$Eh.set(i5, t4);
    }
    this.elementStyles = this.finalizeStyles(this.styles);
  }
  static finalizeStyles(s4) {
    const i5 = [];
    if (Array.isArray(s4)) {
      const e4 = new Set(s4.flat(1 / 0).reverse());
      for (const s5 of e4) i5.unshift(c(s5));
    } else void 0 !== s4 && i5.push(c(s4));
    return i5;
  }
  static _$Eu(t3, s4) {
    const i5 = s4.attribute;
    return false === i5 ? void 0 : "string" == typeof i5 ? i5 : "string" == typeof t3 ? t3.toLowerCase() : void 0;
  }
  constructor() {
    super(), this._$Ep = void 0, this.isUpdatePending = false, this.hasUpdated = false, this._$Em = null, this._$Ev();
  }
  _$Ev() {
    this._$ES = new Promise((t3) => this.enableUpdating = t3), this._$AL = /* @__PURE__ */ new Map(), this._$E_(), this.requestUpdate(), this.constructor.l?.forEach((t3) => t3(this));
  }
  addController(t3) {
    (this._$EO ??= /* @__PURE__ */ new Set()).add(t3), void 0 !== this.renderRoot && this.isConnected && t3.hostConnected?.();
  }
  removeController(t3) {
    this._$EO?.delete(t3);
  }
  _$E_() {
    const t3 = /* @__PURE__ */ new Map(), s4 = this.constructor.elementProperties;
    for (const i5 of s4.keys()) this.hasOwnProperty(i5) && (t3.set(i5, this[i5]), delete this[i5]);
    t3.size > 0 && (this._$Ep = t3);
  }
  createRenderRoot() {
    const t3 = this.shadowRoot ?? this.attachShadow(this.constructor.shadowRootOptions);
    return S(t3, this.constructor.elementStyles), t3;
  }
  connectedCallback() {
    this.renderRoot ??= this.createRenderRoot(), this.enableUpdating(true), this._$EO?.forEach((t3) => t3.hostConnected?.());
  }
  enableUpdating(t3) {
  }
  disconnectedCallback() {
    this._$EO?.forEach((t3) => t3.hostDisconnected?.());
  }
  attributeChangedCallback(t3, s4, i5) {
    this._$AK(t3, i5);
  }
  _$ET(t3, s4) {
    const i5 = this.constructor.elementProperties.get(t3), e4 = this.constructor._$Eu(t3, i5);
    if (void 0 !== e4 && true === i5.reflect) {
      const h3 = (void 0 !== i5.converter?.toAttribute ? i5.converter : u).toAttribute(s4, i5.type);
      this._$Em = t3, null == h3 ? this.removeAttribute(e4) : this.setAttribute(e4, h3), this._$Em = null;
    }
  }
  _$AK(t3, s4) {
    const i5 = this.constructor, e4 = i5._$Eh.get(t3);
    if (void 0 !== e4 && this._$Em !== e4) {
      const t4 = i5.getPropertyOptions(e4), h3 = "function" == typeof t4.converter ? { fromAttribute: t4.converter } : void 0 !== t4.converter?.fromAttribute ? t4.converter : u;
      this._$Em = e4;
      const r4 = h3.fromAttribute(s4, t4.type);
      this[e4] = r4 ?? this._$Ej?.get(e4) ?? r4, this._$Em = null;
    }
  }
  requestUpdate(t3, s4, i5, e4 = false, h3) {
    if (void 0 !== t3) {
      const r4 = this.constructor;
      if (false === e4 && (h3 = this[t3]), i5 ??= r4.getPropertyOptions(t3), !((i5.hasChanged ?? f)(h3, s4) || i5.useDefault && i5.reflect && h3 === this._$Ej?.get(t3) && !this.hasAttribute(r4._$Eu(t3, i5)))) return;
      this.C(t3, s4, i5);
    }
    false === this.isUpdatePending && (this._$ES = this._$EP());
  }
  C(t3, s4, { useDefault: i5, reflect: e4, wrapped: h3 }, r4) {
    i5 && !(this._$Ej ??= /* @__PURE__ */ new Map()).has(t3) && (this._$Ej.set(t3, r4 ?? s4 ?? this[t3]), true !== h3 || void 0 !== r4) || (this._$AL.has(t3) || (this.hasUpdated || i5 || (s4 = void 0), this._$AL.set(t3, s4)), true === e4 && this._$Em !== t3 && (this._$Eq ??= /* @__PURE__ */ new Set()).add(t3));
  }
  async _$EP() {
    this.isUpdatePending = true;
    try {
      await this._$ES;
    } catch (t4) {
      Promise.reject(t4);
    }
    const t3 = this.scheduleUpdate();
    return null != t3 && await t3, !this.isUpdatePending;
  }
  scheduleUpdate() {
    return this.performUpdate();
  }
  performUpdate() {
    if (!this.isUpdatePending) return;
    if (!this.hasUpdated) {
      if (this.renderRoot ??= this.createRenderRoot(), this._$Ep) {
        for (const [t5, s5] of this._$Ep) this[t5] = s5;
        this._$Ep = void 0;
      }
      const t4 = this.constructor.elementProperties;
      if (t4.size > 0) for (const [s5, i5] of t4) {
        const { wrapped: t5 } = i5, e4 = this[s5];
        true !== t5 || this._$AL.has(s5) || void 0 === e4 || this.C(s5, void 0, i5, e4);
      }
    }
    let t3 = false;
    const s4 = this._$AL;
    try {
      t3 = this.shouldUpdate(s4), t3 ? (this.willUpdate(s4), this._$EO?.forEach((t4) => t4.hostUpdate?.()), this.update(s4)) : this._$EM();
    } catch (s5) {
      throw t3 = false, this._$EM(), s5;
    }
    t3 && this._$AE(s4);
  }
  willUpdate(t3) {
  }
  _$AE(t3) {
    this._$EO?.forEach((t4) => t4.hostUpdated?.()), this.hasUpdated || (this.hasUpdated = true, this.firstUpdated(t3)), this.updated(t3);
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
  shouldUpdate(t3) {
    return true;
  }
  update(t3) {
    this._$Eq &&= this._$Eq.forEach((t4) => this._$ET(t4, this[t4])), this._$EM();
  }
  updated(t3) {
  }
  firstUpdated(t3) {
  }
};
y.elementStyles = [], y.shadowRootOptions = { mode: "open" }, y[d("elementProperties")] = /* @__PURE__ */ new Map(), y[d("finalized")] = /* @__PURE__ */ new Map(), p?.({ ReactiveElement: y }), (a.reactiveElementVersions ??= []).push("2.1.2");

// node_modules/lit-html/lit-html.js
var t2 = globalThis;
var i3 = (t3) => t3;
var s2 = t2.trustedTypes;
var e3 = s2 ? s2.createPolicy("lit-html", { createHTML: (t3) => t3 }) : void 0;
var h2 = "$lit$";
var o3 = `lit$${Math.random().toFixed(9).slice(2)}$`;
var n3 = "?" + o3;
var r3 = `<${n3}>`;
var l2 = document;
var c3 = () => l2.createComment("");
var a2 = (t3) => null === t3 || "object" != typeof t3 && "function" != typeof t3;
var u2 = Array.isArray;
var d2 = (t3) => u2(t3) || "function" == typeof t3?.[Symbol.iterator];
var f2 = "[ 	\n\f\r]";
var v = /<(?:(!--|\/[^a-zA-Z])|(\/?[a-zA-Z][^>\s]*)|(\/?$))/g;
var _ = /-->/g;
var m = />/g;
var p2 = RegExp(`>|${f2}(?:([^\\s"'>=/]+)(${f2}*=${f2}*(?:[^ 	
\f\r"'\`<>=]|("|')|))|$)`, "g");
var g = /'/g;
var $ = /"/g;
var y2 = /^(?:script|style|textarea|title)$/i;
var x = (t3) => (i5, ...s4) => ({ _$litType$: t3, strings: i5, values: s4 });
var b2 = x(1);
var w = x(2);
var T = x(3);
var E = /* @__PURE__ */ Symbol.for("lit-noChange");
var A = /* @__PURE__ */ Symbol.for("lit-nothing");
var C = /* @__PURE__ */ new WeakMap();
var P = l2.createTreeWalker(l2, 129);
function V(t3, i5) {
  if (!u2(t3) || !t3.hasOwnProperty("raw")) throw Error("invalid template strings array");
  return void 0 !== e3 ? e3.createHTML(i5) : i5;
}
var N = (t3, i5) => {
  const s4 = t3.length - 1, e4 = [];
  let n4, l3 = 2 === i5 ? "<svg>" : 3 === i5 ? "<math>" : "", c4 = v;
  for (let i6 = 0; i6 < s4; i6++) {
    const s5 = t3[i6];
    let a3, u3, d3 = -1, f3 = 0;
    for (; f3 < s5.length && (c4.lastIndex = f3, u3 = c4.exec(s5), null !== u3); ) f3 = c4.lastIndex, c4 === v ? "!--" === u3[1] ? c4 = _ : void 0 !== u3[1] ? c4 = m : void 0 !== u3[2] ? (y2.test(u3[2]) && (n4 = RegExp("</" + u3[2], "g")), c4 = p2) : void 0 !== u3[3] && (c4 = p2) : c4 === p2 ? ">" === u3[0] ? (c4 = n4 ?? v, d3 = -1) : void 0 === u3[1] ? d3 = -2 : (d3 = c4.lastIndex - u3[2].length, a3 = u3[1], c4 = void 0 === u3[3] ? p2 : '"' === u3[3] ? $ : g) : c4 === $ || c4 === g ? c4 = p2 : c4 === _ || c4 === m ? c4 = v : (c4 = p2, n4 = void 0);
    const x2 = c4 === p2 && t3[i6 + 1].startsWith("/>") ? " " : "";
    l3 += c4 === v ? s5 + r3 : d3 >= 0 ? (e4.push(a3), s5.slice(0, d3) + h2 + s5.slice(d3) + o3 + x2) : s5 + o3 + (-2 === d3 ? i6 : x2);
  }
  return [V(t3, l3 + (t3[s4] || "<?>") + (2 === i5 ? "</svg>" : 3 === i5 ? "</math>" : "")), e4];
};
var S2 = class _S {
  constructor({ strings: t3, _$litType$: i5 }, e4) {
    let r4;
    this.parts = [];
    let l3 = 0, a3 = 0;
    const u3 = t3.length - 1, d3 = this.parts, [f3, v2] = N(t3, i5);
    if (this.el = _S.createElement(f3, e4), P.currentNode = this.el.content, 2 === i5 || 3 === i5) {
      const t4 = this.el.content.firstChild;
      t4.replaceWith(...t4.childNodes);
    }
    for (; null !== (r4 = P.nextNode()) && d3.length < u3; ) {
      if (1 === r4.nodeType) {
        if (r4.hasAttributes()) for (const t4 of r4.getAttributeNames()) if (t4.endsWith(h2)) {
          const i6 = v2[a3++], s4 = r4.getAttribute(t4).split(o3), e5 = /([.?@])?(.*)/.exec(i6);
          d3.push({ type: 1, index: l3, name: e5[2], strings: s4, ctor: "." === e5[1] ? I : "?" === e5[1] ? L : "@" === e5[1] ? z : H }), r4.removeAttribute(t4);
        } else t4.startsWith(o3) && (d3.push({ type: 6, index: l3 }), r4.removeAttribute(t4));
        if (y2.test(r4.tagName)) {
          const t4 = r4.textContent.split(o3), i6 = t4.length - 1;
          if (i6 > 0) {
            r4.textContent = s2 ? s2.emptyScript : "";
            for (let s4 = 0; s4 < i6; s4++) r4.append(t4[s4], c3()), P.nextNode(), d3.push({ type: 2, index: ++l3 });
            r4.append(t4[i6], c3());
          }
        }
      } else if (8 === r4.nodeType) if (r4.data === n3) d3.push({ type: 2, index: l3 });
      else {
        let t4 = -1;
        for (; -1 !== (t4 = r4.data.indexOf(o3, t4 + 1)); ) d3.push({ type: 7, index: l3 }), t4 += o3.length - 1;
      }
      l3++;
    }
  }
  static createElement(t3, i5) {
    const s4 = l2.createElement("template");
    return s4.innerHTML = t3, s4;
  }
};
function M(t3, i5, s4 = t3, e4) {
  if (i5 === E) return i5;
  let h3 = void 0 !== e4 ? s4._$Co?.[e4] : s4._$Cl;
  const o5 = a2(i5) ? void 0 : i5._$litDirective$;
  return h3?.constructor !== o5 && (h3?._$AO?.(false), void 0 === o5 ? h3 = void 0 : (h3 = new o5(t3), h3._$AT(t3, s4, e4)), void 0 !== e4 ? (s4._$Co ??= [])[e4] = h3 : s4._$Cl = h3), void 0 !== h3 && (i5 = M(t3, h3._$AS(t3, i5.values), h3, e4)), i5;
}
var R = class {
  constructor(t3, i5) {
    this._$AV = [], this._$AN = void 0, this._$AD = t3, this._$AM = i5;
  }
  get parentNode() {
    return this._$AM.parentNode;
  }
  get _$AU() {
    return this._$AM._$AU;
  }
  u(t3) {
    const { el: { content: i5 }, parts: s4 } = this._$AD, e4 = (t3?.creationScope ?? l2).importNode(i5, true);
    P.currentNode = e4;
    let h3 = P.nextNode(), o5 = 0, n4 = 0, r4 = s4[0];
    for (; void 0 !== r4; ) {
      if (o5 === r4.index) {
        let i6;
        2 === r4.type ? i6 = new k(h3, h3.nextSibling, this, t3) : 1 === r4.type ? i6 = new r4.ctor(h3, r4.name, r4.strings, this, t3) : 6 === r4.type && (i6 = new Z(h3, this, t3)), this._$AV.push(i6), r4 = s4[++n4];
      }
      o5 !== r4?.index && (h3 = P.nextNode(), o5++);
    }
    return P.currentNode = l2, e4;
  }
  p(t3) {
    let i5 = 0;
    for (const s4 of this._$AV) void 0 !== s4 && (void 0 !== s4.strings ? (s4._$AI(t3, s4, i5), i5 += s4.strings.length - 2) : s4._$AI(t3[i5])), i5++;
  }
};
var k = class _k {
  get _$AU() {
    return this._$AM?._$AU ?? this._$Cv;
  }
  constructor(t3, i5, s4, e4) {
    this.type = 2, this._$AH = A, this._$AN = void 0, this._$AA = t3, this._$AB = i5, this._$AM = s4, this.options = e4, this._$Cv = e4?.isConnected ?? true;
  }
  get parentNode() {
    let t3 = this._$AA.parentNode;
    const i5 = this._$AM;
    return void 0 !== i5 && 11 === t3?.nodeType && (t3 = i5.parentNode), t3;
  }
  get startNode() {
    return this._$AA;
  }
  get endNode() {
    return this._$AB;
  }
  _$AI(t3, i5 = this) {
    t3 = M(this, t3, i5), a2(t3) ? t3 === A || null == t3 || "" === t3 ? (this._$AH !== A && this._$AR(), this._$AH = A) : t3 !== this._$AH && t3 !== E && this._(t3) : void 0 !== t3._$litType$ ? this.$(t3) : void 0 !== t3.nodeType ? this.T(t3) : d2(t3) ? this.k(t3) : this._(t3);
  }
  O(t3) {
    return this._$AA.parentNode.insertBefore(t3, this._$AB);
  }
  T(t3) {
    this._$AH !== t3 && (this._$AR(), this._$AH = this.O(t3));
  }
  _(t3) {
    this._$AH !== A && a2(this._$AH) ? this._$AA.nextSibling.data = t3 : this.T(l2.createTextNode(t3)), this._$AH = t3;
  }
  $(t3) {
    const { values: i5, _$litType$: s4 } = t3, e4 = "number" == typeof s4 ? this._$AC(t3) : (void 0 === s4.el && (s4.el = S2.createElement(V(s4.h, s4.h[0]), this.options)), s4);
    if (this._$AH?._$AD === e4) this._$AH.p(i5);
    else {
      const t4 = new R(e4, this), s5 = t4.u(this.options);
      t4.p(i5), this.T(s5), this._$AH = t4;
    }
  }
  _$AC(t3) {
    let i5 = C.get(t3.strings);
    return void 0 === i5 && C.set(t3.strings, i5 = new S2(t3)), i5;
  }
  k(t3) {
    u2(this._$AH) || (this._$AH = [], this._$AR());
    const i5 = this._$AH;
    let s4, e4 = 0;
    for (const h3 of t3) e4 === i5.length ? i5.push(s4 = new _k(this.O(c3()), this.O(c3()), this, this.options)) : s4 = i5[e4], s4._$AI(h3), e4++;
    e4 < i5.length && (this._$AR(s4 && s4._$AB.nextSibling, e4), i5.length = e4);
  }
  _$AR(t3 = this._$AA.nextSibling, s4) {
    for (this._$AP?.(false, true, s4); t3 !== this._$AB; ) {
      const s5 = i3(t3).nextSibling;
      i3(t3).remove(), t3 = s5;
    }
  }
  setConnected(t3) {
    void 0 === this._$AM && (this._$Cv = t3, this._$AP?.(t3));
  }
};
var H = class {
  get tagName() {
    return this.element.tagName;
  }
  get _$AU() {
    return this._$AM._$AU;
  }
  constructor(t3, i5, s4, e4, h3) {
    this.type = 1, this._$AH = A, this._$AN = void 0, this.element = t3, this.name = i5, this._$AM = e4, this.options = h3, s4.length > 2 || "" !== s4[0] || "" !== s4[1] ? (this._$AH = Array(s4.length - 1).fill(new String()), this.strings = s4) : this._$AH = A;
  }
  _$AI(t3, i5 = this, s4, e4) {
    const h3 = this.strings;
    let o5 = false;
    if (void 0 === h3) t3 = M(this, t3, i5, 0), o5 = !a2(t3) || t3 !== this._$AH && t3 !== E, o5 && (this._$AH = t3);
    else {
      const e5 = t3;
      let n4, r4;
      for (t3 = h3[0], n4 = 0; n4 < h3.length - 1; n4++) r4 = M(this, e5[s4 + n4], i5, n4), r4 === E && (r4 = this._$AH[n4]), o5 ||= !a2(r4) || r4 !== this._$AH[n4], r4 === A ? t3 = A : t3 !== A && (t3 += (r4 ?? "") + h3[n4 + 1]), this._$AH[n4] = r4;
    }
    o5 && !e4 && this.j(t3);
  }
  j(t3) {
    t3 === A ? this.element.removeAttribute(this.name) : this.element.setAttribute(this.name, t3 ?? "");
  }
};
var I = class extends H {
  constructor() {
    super(...arguments), this.type = 3;
  }
  j(t3) {
    this.element[this.name] = t3 === A ? void 0 : t3;
  }
};
var L = class extends H {
  constructor() {
    super(...arguments), this.type = 4;
  }
  j(t3) {
    this.element.toggleAttribute(this.name, !!t3 && t3 !== A);
  }
};
var z = class extends H {
  constructor(t3, i5, s4, e4, h3) {
    super(t3, i5, s4, e4, h3), this.type = 5;
  }
  _$AI(t3, i5 = this) {
    if ((t3 = M(this, t3, i5, 0) ?? A) === E) return;
    const s4 = this._$AH, e4 = t3 === A && s4 !== A || t3.capture !== s4.capture || t3.once !== s4.once || t3.passive !== s4.passive, h3 = t3 !== A && (s4 === A || e4);
    e4 && this.element.removeEventListener(this.name, this, s4), h3 && this.element.addEventListener(this.name, this, t3), this._$AH = t3;
  }
  handleEvent(t3) {
    "function" == typeof this._$AH ? this._$AH.call(this.options?.host ?? this.element, t3) : this._$AH.handleEvent(t3);
  }
};
var Z = class {
  constructor(t3, i5, s4) {
    this.element = t3, this.type = 6, this._$AN = void 0, this._$AM = i5, this.options = s4;
  }
  get _$AU() {
    return this._$AM._$AU;
  }
  _$AI(t3) {
    M(this, t3);
  }
};
var B = t2.litHtmlPolyfillSupport;
B?.(S2, k), (t2.litHtmlVersions ??= []).push("3.3.3");
var D = (t3, i5, s4) => {
  const e4 = s4?.renderBefore ?? i5;
  let h3 = e4._$litPart$;
  if (void 0 === h3) {
    const t4 = s4?.renderBefore ?? null;
    e4._$litPart$ = h3 = new k(i5.insertBefore(c3(), t4), t4, void 0, s4 ?? {});
  }
  return h3._$AI(t3), h3;
};

// node_modules/lit-element/lit-element.js
var s3 = globalThis;
var i4 = class extends y {
  constructor() {
    super(...arguments), this.renderOptions = { host: this }, this._$Do = void 0;
  }
  createRenderRoot() {
    const t3 = super.createRenderRoot();
    return this.renderOptions.renderBefore ??= t3.firstChild, t3;
  }
  update(t3) {
    const r4 = this.render();
    this.hasUpdated || (this.renderOptions.isConnected = this.isConnected), super.update(t3), this._$Do = D(r4, this.renderRoot, this.renderOptions);
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

// web/components/filter-url.ts
var emptyFilters = { controls: {}, visualSelections: [] };
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
function filtersToURLParams(config, filters) {
  const params = {};
  for (const [name, definition] of Object.entries(config)) {
    const control = filters.controls?.[name] ?? defaultControl(definition);
    const base = defaultControl(definition);
    switch (definition.type) {
      case "date_range":
        if (!definition.urlParam) break;
        if (control.from || control.to || control.preset === "custom") {
          params[definition.urlParam] = "custom";
          addString(params, definition.fromURLParam, control.from);
          addString(params, definition.toURLParam, control.to);
          break;
        }
        if (control.preset && control.preset !== base.preset) {
          params[definition.urlParam] = control.preset;
        }
        break;
      case "multi_select":
        if (definition.urlParam && (control.values ?? []).length > 0) {
          params[definition.urlParam] = [...control.values ?? []].filter(Boolean).sort();
        }
        break;
      case "text": {
        const value = (control.value ?? "").trim();
        if (!definition.urlParam || !value) break;
        params[definition.urlParam] = value;
        if (definition.operatorURLParam && control.operator && control.operator !== base.operator) {
          params[definition.operatorURLParam] = control.operator;
        }
        break;
      }
    }
  }
  return params;
}
function addString(params, key, value) {
  const trimmed = (value ?? "").trim();
  if (key && trimmed) {
    params[key] = trimmed;
  }
}

// web/components/filter-card.ts
var filterCardStyles = i`
    :host {
      display: block;
      height: 100%;
      color: var(--fgColor-default);
      font-family: system-ui, -apple-system, BlinkMacSystemFont, 'Segoe UI', sans-serif;
    }

    .card {
      position: relative;
      display: grid;
      height: 100%;
      min-width: 0;
      align-content: center;
      gap: 4px;
      border: 0;
      background: transparent;
      padding: 8px 10px;
      box-sizing: border-box;
    }

    .label {
      overflow: hidden;
      color: var(--fgColor-muted);
      text-overflow: ellipsis;
      white-space: nowrap;
      font-size: 0.58rem;
      font-weight: 900;
      line-height: 1;
      text-transform: uppercase;
    }

    .trigger {
      display: flex;
      min-width: 0;
      align-items: center;
      justify-content: space-between;
      gap: 8px;
      border: 0;
      background: transparent;
      color: var(--fgColor-default);
      cursor: pointer;
      padding: 0;
      text-align: left;
      font: inherit;
    }

    .value {
      min-width: 0;
      overflow: hidden;
      text-overflow: ellipsis;
      white-space: nowrap;
      font-size: 0.82rem;
      font-weight: 850;
      line-height: 1.15;
    }

    .chevron {
      flex: 0 0 auto;
      color: var(--fgColor-muted);
      font-size: 0.68rem;
      font-weight: 900;
    }

    .popover {
      position: absolute;
      top: calc(100% + 6px);
      left: 0;
      z-index: 30;
      display: grid;
      width: min(260px, max(100%, 220px));
      gap: 7px;
      border: 1px solid var(--borderColor-default);
      border-radius: 6px;
      background: var(--overlay-bgColor, var(--bgColor-default));
      box-shadow: var(--shadow-floating-small, 0 8px 24px rgb(0 0 0 / 18%));
      padding: 8px;
    }

    button,
    input,
    select {
      font: inherit;
    }

    input,
    select {
      width: 100%;
      min-width: 0;
      min-height: 27px;
      border: 1px solid var(--borderColor-default);
      border-radius: 4px;
      background: var(--control-bgColor-rest);
      color: var(--fgColor-default);
      padding: 0 7px;
      font-size: 0.68rem;
      font-weight: 650;
      outline-offset: 2px;
      box-sizing: border-box;
    }

    input:focus,
    select:focus,
    button:focus-visible {
      outline: 2px solid var(--ld-accent);
    }

    .chips {
      display: grid;
      grid-template-columns: repeat(2, minmax(0, 1fr));
      gap: 4px;
    }

    .chip,
    .action {
      min-height: 27px;
      border: 1px solid var(--borderColor-default);
      border-radius: 4px;
      background: var(--control-bgColor-rest);
      color: var(--fgColor-default);
      cursor: pointer;
      padding: 0 7px;
      font-size: 0.64rem;
      font-weight: 850;
    }

    .chip.custom {
      grid-column: 1 / -1;
    }

    .chip[aria-pressed='true'] {
      border-color: var(--ld-accent);
      background: color-mix(in srgb, var(--ld-accent) 20%, var(--control-bgColor-rest));
    }

    .date-row,
    .actions {
      display: grid;
      grid-template-columns: 1fr 1fr;
      gap: 6px;
    }

    .actions.three {
      grid-template-columns: 1fr 1fr 1fr;
    }

    .action.primary {
      border-color: var(--button-primary-bgColor-rest);
      background: var(--button-primary-bgColor-rest);
      color: var(--button-primary-fgColor-rest);
    }

    .checks {
      display: grid;
      max-height: 152px;
      gap: 2px;
      overflow: auto;
    }

    .check {
      display: flex;
      min-width: 0;
      align-items: center;
      gap: 6px;
      border-radius: 4px;
      padding: 4px;
      color: var(--fgColor-default);
      font-size: 0.68rem;
      font-weight: 750;
    }

    .check:hover {
      background: var(--bgColor-muted);
    }

    .check input {
      width: 13px;
      height: 13px;
      min-height: 0;
      accent-color: var(--ld-accent);
    }

    .empty {
      color: var(--fgColor-muted);
      font-size: 0.66rem;
      font-weight: 750;
      padding: 4px;
    }
  `;
var FilterCard = class extends HTMLElement {
  constructor() {
    super(...arguments);
    this.open = false;
    this.search = "";
    this.draftFrom = "";
    this.draftTo = "";
    this.customDate = false;
  }
  static {
    this.observedAttributes = ["filter-id", "config", "filters", "options", "loading"];
  }
  connectedCallback() {
    if (!this.shadowRoot) this.attachShadow({ mode: "open" });
    this.renderCard();
  }
  attributeChangedCallback() {
    this.renderCard();
  }
  renderCard() {
    if (!this.shadowRoot) return;
    D(this.template(), this.shadowRoot);
  }
  template() {
    const definition = this.definition();
    if (!definition) return b2`<style>${filterCardStyles}</style><slot></slot>`;
    const control = this.control(definition);
    return b2`
      <style>${filterCardStyles}</style>
      <section class="card" aria-label=${definition.label}>
        <div class="label">${definition.label}</div>
        <button class="trigger" type="button" ?disabled=${this.isLoading()} aria-expanded=${String(this.open)} @click=${() => this.toggle(definition, control)}>
          <span class="value">${this.summary(definition, control)}</span>
          <span class="chevron" aria-hidden="true">▾</span>
        </button>
        ${this.open ? this.renderPopover(definition, control) : A}
      </section>
    `;
  }
  renderPopover(definition, control) {
    switch (definition.type) {
      case "date_range":
        return this.renderDate(definition, control);
      case "multi_select":
        return this.renderMulti(definition, control);
      case "text":
        return this.renderText(definition, control);
      default:
        return A;
    }
  }
  renderDate(definition, control) {
    const preset = control.preset || definition.default?.preset || "all";
    const presets = [...definition.presets ?? []];
    if (definition.custom) presets.push({ value: "custom", label: "Custom" });
    const custom = this.customDate || preset === "custom" || Boolean(control.from || control.to);
    return b2`
      <div class="popover">
        <div class="chips">
          ${presets.map((item) => b2`
            <button
              class=${`chip ${item.value === "custom" ? "custom" : ""}`}
              type="button"
              aria-pressed=${String((custom ? "custom" : preset) === item.value)}
              @click=${() => this.pickDatePreset(control, item.value)}
            >${presetLabel(item)}</button>
          `)}
        </div>
        ${custom ? b2`
          <div class="date-row">
            <input type="date" aria-label="${definition.label} from" .value=${this.draftFrom} @input=${(event) => this.setDraft("from", event)} />
            <input type="date" aria-label="${definition.label} to" .value=${this.draftTo} @input=${(event) => this.setDraft("to", event)} />
          </div>
          <div class="actions three">
            <button class="action" type="button" @click=${() => this.clear()}>Clear</button>
            <button class="action" type="button" @click=${() => this.close()}>Cancel</button>
            <button class="action primary" type="button" @click=${() => this.applyDate(control)}>Apply</button>
          </div>
        ` : A}
      </div>
    `;
  }
  renderMulti(definition, control) {
    const selected = new Set(control.values ?? []);
    const search = this.search.toLowerCase();
    const options = (this.currentOptions()[this.currentFilterId()] ?? []).filter((option) => option.label.toLowerCase().includes(search) || option.value.toLowerCase().includes(search));
    return b2`
      <div class="popover">
        <input type="search" placeholder="Search ${definition.label.toLowerCase()}..." .value=${this.search} @input=${(event) => this.setSearch(event)} />
        <div class="checks">
          ${options.length === 0 ? b2`<div class="empty">No values loaded</div>` : A}
          ${options.map((option) => b2`
            <label class="check">
              <input type="checkbox" .checked=${selected.has(option.value)} @change=${() => this.toggleValue(control, option.value)} />
              <span>${option.label}</span>
            </label>
          `)}
        </div>
        <div class="actions">
          <button class="action" type="button" @click=${() => this.clear()}>Clear</button>
          <button class="action" type="button" @click=${() => this.close()}>Close</button>
        </div>
      </div>
    `;
  }
  renderText(definition, control) {
    return b2`
      <div class="popover">
        <select aria-label="${definition.label} operator" .value=${control.operator ?? definition.defaultOperator ?? "contains"} @change=${(event) => this.update({ ...control, type: "text", operator: event.currentTarget.value })}>
          ${(definition.operators ?? ["contains"]).map((operator) => b2`<option value=${operator}>${operatorLabel(operator)}</option>`)}
        </select>
        <input type="search" placeholder="Search..." .value=${control.value ?? ""} @input=${(event) => this.update({ ...control, type: "text", value: event.currentTarget.value })} />
        <div class="actions">
          <button class="action" type="button" @click=${() => this.clear()}>Clear</button>
          <button class="action" type="button" @click=${() => this.close()}>Close</button>
        </div>
      </div>
    `;
  }
  definition() {
    return this.currentConfig()[this.currentFilterId()];
  }
  control(definition) {
    return this.currentFilters().controls?.[this.currentFilterId()] ?? defaultControl(definition);
  }
  summary(definition, control) {
    switch (definition.type) {
      case "date_range":
        return dateSummary(definition, control);
      case "multi_select": {
        const count = control.values?.length ?? 0;
        if (count === 0) return allValuesLabel(definition);
        if (count === 1) return control.values?.[0] ?? "";
        return `${count} selected`;
      }
      case "text":
        return control.value?.trim() || `Any ${definition.label.toLowerCase()}`;
      default:
        return definition.label;
    }
  }
  toggle(definition, control) {
    this.open = !this.open;
    if (this.open && definition.type === "date_range") {
      this.draftFrom = control.from ?? "";
      this.draftTo = control.to ?? "";
      this.customDate = control.preset === "custom" || Boolean(control.from || control.to);
    }
    this.renderCard();
  }
  pickDatePreset(control, value) {
    if (value === "custom") {
      this.draftFrom = control.from ?? "";
      this.draftTo = control.to ?? "";
      this.customDate = true;
      this.renderCard();
      return;
    }
    this.customDate = false;
    this.update({ ...control, type: "date_range", preset: value, from: "", to: "" });
    this.open = false;
    this.renderCard();
  }
  setDraft(key, event) {
    const value = event.currentTarget.value;
    if (key === "from") this.draftFrom = value;
    if (key === "to") this.draftTo = value;
    if (this.draftFrom && this.draftTo && this.draftTo < this.draftFrom) {
      const from = this.draftTo;
      this.draftTo = this.draftFrom;
      this.draftFrom = from;
    }
    this.renderCard();
  }
  setSearch(event) {
    this.search = event.currentTarget.value;
    this.renderCard();
  }
  applyDate(control) {
    this.update({ ...control, type: "date_range", preset: "custom", from: this.draftFrom, to: this.draftTo });
    this.customDate = false;
    this.open = false;
    this.renderCard();
  }
  toggleValue(control, value) {
    const selected = new Set(control.values ?? []);
    if (selected.has(value)) {
      selected.delete(value);
    } else {
      selected.add(value);
    }
    this.update({ ...control, type: "multi_select", operator: "in", values: [...selected].sort() });
    this.renderCard();
  }
  clear() {
    const definition = this.definition();
    if (!definition) return;
    this.draftFrom = "";
    this.draftTo = "";
    this.search = "";
    this.customDate = false;
    this.update(defaultControl(definition));
    this.open = false;
    this.renderCard();
  }
  update(control) {
    const filtersSignal = this.currentFilters();
    const filters = {
      controls: { ...filtersSignal.controls ?? {}, [this.currentFilterId()]: control },
      visualSelections: [...filtersSignal.visualSelections ?? []]
    };
    const config = this.currentConfig();
    this.dispatchEvent(new CustomEvent("ld-filters-change", {
      detail: { filters, urlParams: filtersToURLParams(config, filters) },
      bubbles: true,
      composed: true
    }));
  }
  currentFilterId() {
    return this.getAttribute("filter-id") || "";
  }
  currentConfig() {
    return readJSONAttribute(this, "config", {});
  }
  currentFilters() {
    return readJSONAttribute(this, "filters", emptyFilters);
  }
  currentOptions() {
    return readJSONAttribute(this, "options", {});
  }
  isLoading() {
    const loading = this.getAttribute("loading");
    return loading !== null && loading !== "false";
  }
  close() {
    this.open = false;
    this.renderCard();
  }
};
function presetLabel(preset) {
  if (preset.value === "all") return "All";
  if (preset.relativeDays) return `${preset.relativeDays}d`;
  return preset.label;
}
function dateSummary(definition, control) {
  if (control.from || control.to) {
    if (control.from && control.to) return `${formatDate(control.from)} - ${formatDate(control.to)}`;
    if (control.from) return `From ${formatDate(control.from)}`;
    return `Until ${formatDate(control.to ?? "")}`;
  }
  const preset = control.preset || definition.default?.preset || "all";
  return (definition.presets ?? []).find((item) => item.value === preset)?.label ?? "Custom range";
}
function allValuesLabel(definition) {
  const label = definition.label.toLowerCase();
  if (label === "state") return "All states";
  if (label.endsWith("y")) return `All ${label.slice(0, -1)}ies`;
  if (label.endsWith("s")) return `All ${label}`;
  return `All ${label}s`;
}
function formatDate(value) {
  const [year, month, day] = value.split("-").map((part) => Number(part));
  if (!year || !month || !day) return value;
  return new Intl.DateTimeFormat(void 0, { month: "short", day: "numeric", year: "numeric" }).format(new Date(year, month - 1, day));
}
function operatorLabel(operator) {
  switch (operator) {
    case "equals":
      return "Equals";
    case "starts_with":
      return "Starts with";
    case "not_contains":
      return "Does not contain";
    default:
      return "Contains";
  }
}
function readJSONAttribute(element, name, fallback) {
  const value = element.getAttribute(name);
  if (!value) return fallback;
  try {
    return JSON.parse(value);
  } catch {
    return fallback;
  }
}
if (!window.customElements.get("ld-filter-card")) window.customElements.define("ld-filter-card", FilterCard);
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
*/
