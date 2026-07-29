import { describe, expect, test } from "bun:test";
import { readFile } from "node:fs/promises";
import { resolve } from "node:path";

import {
  addWindowsManagedDeployment,
  desktopInstallerContract,
} from "../installer-contract.js";

describe("desktop installer contract", () => {
  test("selects machine-managed native package formats", () => {
    expect(desktopInstallerContract).toMatchObject({
      installationScope: "per-machine",
      formats: {
        darwin: "pkg",
        linux: "deb",
        win32: "msi",
      },
      managedPolicy: {
        retainOnUninstall: true,
        win32KnownFolder: "FOLDERID_ProgramData",
      },
      protocol: {
        scheme: "leapview-desktop",
        argumentToken: "%1",
      },
    });
  });

  test("adds a transactional machine protocol and protected ProgramData directory", () => {
    const template = [
      "<Product>",
      'InstallerVersion="405"',
      "      <!-- Desktop -->",
      '    <Feature Id="Complete"',
      '        <ComponentRef Id="PurgeOnUninstall" />',
      "</Product>",
    ].join("\n");
    const configured = addWindowsManagedDeployment(template);
    expect(configured).toContain('Id="CommonAppDataFolder"');
    expect(configured).toContain('InstallerVersion="500"');
    expect(configured).toContain('Id="LeapViewPolicyDirectory"');
    expect(configured).toContain(
      'Sddl="D:P(A;OICI;GA;;;SY)(A;OICI;GA;;;BA)(A;OICI;GRGX;;;BU)"',
    );
    expect(configured).toContain(
      'Key="Software\\Classes\\leapview-desktop"',
    );
    expect(configured).toContain(
      'Value="&quot;[APPLICATIONROOTDIRECTORY]LeapView.exe&quot; &quot;%1&quot;"',
    );
    expect(configured).toContain('ForceDeleteOnUninstall="yes"');
    expect(configured).toContain(
      '<ComponentRef Id="LeapViewDesktopProtocol" />',
    );
  });

  test("fails closed when the pinned maker template changes", () => {
    expect(() => addWindowsManagedDeployment("<Product/>")).toThrow(
      /pinned WiX template/,
    );
  });

  test("patches the exact template shipped by the pinned WiX dependency", async () => {
    const template = await readFile(
      resolve(
        import.meta.dirname,
        "../node_modules/electron-wix-msi/static/wix.xml",
      ),
      "utf8",
    );
    const configured = addWindowsManagedDeployment(template);
    expect(configured).toContain('InstallerVersion="500"');
    expect(configured).toContain(
      '<ComponentRef Id="LeapViewManagedPolicyDirectory" />',
    );
    expect(configured).toContain(
      '<ComponentRef Id="LeapViewDesktopProtocol" />',
    );
  });

  test("POSIX installer scripts preserve policy and repair root-only ownership", async () => {
    const root = resolve(import.meta.dirname, "..");
    const [macPreinstall, macPostinstall, linuxPreinstall, linuxPostinstall] =
      await Promise.all([
        readFile(
          resolve(root, "installer/macos/scripts/preinstall"),
          "utf8",
        ),
        readFile(
          resolve(root, "installer/macos/scripts/postinstall"),
          "utf8",
        ),
        readFile(
          resolve(root, "installer/linux/preinst"),
          "utf8",
        ),
        readFile(
          resolve(root, "installer/linux/postinst"),
          "utf8",
        ),
      ]);
    for (const script of [
      macPreinstall,
      macPostinstall,
      linuxPreinstall,
      linuxPostinstall,
    ]) {
      expect(script).toContain("desktop-policy.json");
      expect(script).toContain("policy location must not be a symlink");
      expect(script).not.toContain("rm ");
    }
    expect(macPostinstall).toContain("chown root:wheel");
    expect(macPostinstall).toContain("chmod 0644");
    expect(linuxPostinstall).toContain("chown root:root");
    expect(linuxPostinstall).toContain("chmod 0644");
  });
});
