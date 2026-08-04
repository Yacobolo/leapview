const WINDOWS_DIRECTORY_ANCHOR = `      <!-- Desktop -->`;
const WINDOWS_COMPONENT_ANCHOR = `    <Feature Id="Complete"`;
const WINDOWS_COMPONENT_REFERENCE_ANCHOR =
  `        <ComponentRef Id="PurgeOnUninstall" />`;
const WINDOWS_INSTALLER_VERSION_ANCHOR = `InstallerVersion="405"`;

const windowsPolicyDirectory = `      <Directory Id="CommonAppDataFolder">
        <Directory Id="LeapViewPolicyDirectory" Name="LeapView"/>
      </Directory>

`;

const windowsManagedComponents = `    <DirectoryRef Id="LeapViewPolicyDirectory">
      <Component Id="LeapViewManagedPolicyDirectory"
                 Guid="5A2B3A94-9D9A-42B6-9352-499209322837"
                 Permanent="yes"
                 NeverOverwrite="yes"
                 Win64="yes">
        <CreateFolder>
          <PermissionEx Sddl="D:P(A;OICI;GA;;;SY)(A;OICI;GA;;;BA)(A;OICI;GRGX;;;BU)"/>
        </CreateFolder>
      </Component>
    </DirectoryRef>

    <DirectoryRef Id="APPLICATIONROOTDIRECTORY">
      <Component Id="LeapViewDesktopProtocol"
                 Guid="34601615-388E-49DF-84DF-EB6722B707C5"
                 Win64="yes">
        <RegistryKey Root="HKLM"
                     Key="Software\\Classes\\leapview-desktop"
                     ForceCreateOnInstall="yes"
                     ForceDeleteOnUninstall="yes">
          <RegistryValue Type="string"
                         Value="URL:LeapView Desktop Protocol"
                         KeyPath="yes"/>
          <RegistryValue Name="URL Protocol" Type="string" Value=""/>
          <RegistryKey Key="shell\\open\\command">
            <RegistryValue Type="string"
                           Value="&quot;[APPLICATIONROOTDIRECTORY]LeapView.exe&quot; &quot;%1&quot;"/>
          </RegistryKey>
        </RegistryKey>
      </Component>
    </DirectoryRef>

`;

export const consumerDistributionContract = {
  schemaVersion: 2,
  channel: "consumer-v1",
  platforms: {
    darwin: {
      installer: "dmg",
      updateArtifacts: ["zip"],
      updateMechanism: "squirrel-mac",
      scope: "user-installed",
    },
    linux: {
      installer: "deb",
      updateArtifacts: [],
      updateMechanism: "apt",
      scope: "system-package-manager",
    },
    win32: {
      installer: "exe",
      updateArtifacts: ["nupkg", "RELEASES"],
      updateMechanism: "squirrel-windows",
      scope: "per-user",
    },
  },
  protocol: {
    scheme: "leapview-desktop",
    argumentToken: "%1",
  },
} as const;

export const managedInstallerGroundwork = {
  schemaVersion: 1,
  supportedInConsumerV1: false,
  installationScope: "per-machine",
  formats: {
    darwin: "pkg",
    linux: "deb",
    win32: "msi",
  },
  managedPolicy: {
    darwin: "/Library/Application Support/LeapView/desktop-policy.json",
    linux: "/etc/leapview/desktop-policy.json",
    win32KnownFolder: "FOLDERID_ProgramData",
    relativePath: String.raw`LeapView\desktop-policy.json`,
    retainOnUninstall: true,
  },
  protocol: {
    scheme: "leapview-desktop",
    argumentToken: "%1",
  },
} as const;

export function addWindowsManagedDeployment(
  wixTemplate: string,
): string {
  const requiredAnchors = [
    WINDOWS_DIRECTORY_ANCHOR,
    WINDOWS_COMPONENT_ANCHOR,
    WINDOWS_COMPONENT_REFERENCE_ANCHOR,
    WINDOWS_INSTALLER_VERSION_ANCHOR,
  ];
  for (const anchor of requiredAnchors) {
    if (wixTemplate.split(anchor).length !== 2) {
      throw new Error(
        `pinned WiX template does not contain exactly one ${anchor}`,
      );
    }
  }
  return wixTemplate
    .replace(WINDOWS_INSTALLER_VERSION_ANCHOR, `InstallerVersion="500"`)
    .replace(
      WINDOWS_DIRECTORY_ANCHOR,
      `${windowsPolicyDirectory}${WINDOWS_DIRECTORY_ANCHOR}`,
    )
    .replace(
      WINDOWS_COMPONENT_ANCHOR,
      `${windowsManagedComponents}${WINDOWS_COMPONENT_ANCHOR}`,
    )
    .replace(
      WINDOWS_COMPONENT_REFERENCE_ANCHOR,
      `${WINDOWS_COMPONENT_REFERENCE_ANCHOR}
        <ComponentRef Id="LeapViewManagedPolicyDirectory" />
        <ComponentRef Id="LeapViewDesktopProtocol" />`,
    );
}
