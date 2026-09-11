// Hand-rolled base64url <-> ArrayBuffer conversion for the WebAuthn ceremony
// payloads. The backend (go-webauthn) serializes challenges and credential
// IDs as base64url strings — the same convention used by every other
// WebAuthn server library — so these helpers translate between that wire
// format and the ArrayBuffers navigator.credentials.create()/.get() expect.
// Implemented directly rather than relying on
// PublicKeyCredential.parseCreationOptionsFromJSON/toJSON so this works on
// browsers that don't yet ship those (Safari only gained them in 2024).

function base64urlToBuffer(base64url: string): ArrayBuffer {
  const padded = base64url.replace(/-/g, "+").replace(/_/g, "/");
  const padding = "=".repeat((4 - (padded.length % 4)) % 4);
  const binary = atob(padded + padding);
  const bytes = new Uint8Array(binary.length);
  for (let i = 0; i < binary.length; i++) bytes[i] = binary.charCodeAt(i);
  return bytes.buffer;
}

function bufferToBase64url(buffer: ArrayBuffer): string {
  const bytes = new Uint8Array(buffer);
  let binary = "";
  for (const byte of bytes) binary += String.fromCharCode(byte);
  return btoa(binary).replace(/\+/g, "-").replace(/\//g, "_").replace(/=+$/, "");
}

interface ServerCredentialDescriptor {
  id: string;
  type: string;
  transports?: string[];
}

// Minimal shapes for the JSON go-webauthn sends over the wire — user.id and
// challenge as base64url strings rather than the BufferSource the browser's
// own PublicKeyCredentialCreationOptions type expects for other fields.
interface ServerCreationOptions {
  rp: PublicKeyCredentialRpEntity;
  user: { id: string; name: string; displayName: string };
  challenge: string;
  pubKeyCredParams: PublicKeyCredentialParameters[];
  timeout?: number;
  excludeCredentials?: ServerCredentialDescriptor[];
  authenticatorSelection?: AuthenticatorSelectionCriteria;
  attestation?: AttestationConveyancePreference;
}

interface ServerRequestOptions {
  challenge: string;
  timeout?: number;
  rpId?: string;
  allowCredentials?: ServerCredentialDescriptor[];
  userVerification?: UserVerificationRequirement;
}

function toCredentialDescriptors(list?: ServerCredentialDescriptor[]): PublicKeyCredentialDescriptor[] | undefined {
  if (!list) return undefined;
  return list.map((c) => ({
    id: base64urlToBuffer(c.id),
    type: "public-key",
    transports: c.transports as AuthenticatorTransport[] | undefined,
  }));
}

// creationOptionsFromServer converts the `publicKey` object from a
// register/begin response into the shape navigator.credentials.create()
// expects.
export function creationOptionsFromServer(publicKey: ServerCreationOptions): CredentialCreationOptions {
  return {
    publicKey: {
      ...publicKey,
      challenge: base64urlToBuffer(publicKey.challenge),
      user: { ...publicKey.user, id: base64urlToBuffer(publicKey.user.id) },
      excludeCredentials: toCredentialDescriptors(publicKey.excludeCredentials),
    },
  };
}

// requestOptionsFromServer converts the `publicKey` object from a
// reveal/begin response into the shape navigator.credentials.get() expects.
export function requestOptionsFromServer(publicKey: ServerRequestOptions): CredentialRequestOptions {
  return {
    publicKey: {
      ...publicKey,
      challenge: base64urlToBuffer(publicKey.challenge),
      allowCredentials: toCredentialDescriptors(publicKey.allowCredentials),
    },
  };
}

// registrationCredentialToJSON serializes a freshly created credential back
// into the standard base64url JSON shape the Go backend's
// protocol.ParseCredentialCreationResponse expects as the finish request
// body.
export function registrationCredentialToJSON(credential: PublicKeyCredential) {
  const response = credential.response as AuthenticatorAttestationResponse;
  return {
    id: credential.id,
    rawId: bufferToBase64url(credential.rawId),
    type: credential.type,
    response: {
      clientDataJSON: bufferToBase64url(response.clientDataJSON),
      attestationObject: bufferToBase64url(response.attestationObject),
      transports: response.getTransports ? response.getTransports() : undefined,
    },
    clientExtensionResults: credential.getClientExtensionResults(),
  };
}

// assertionCredentialToJSON does the same for the response of
// navigator.credentials.get().
export function assertionCredentialToJSON(credential: PublicKeyCredential) {
  const response = credential.response as AuthenticatorAssertionResponse;
  return {
    id: credential.id,
    rawId: bufferToBase64url(credential.rawId),
    type: credential.type,
    response: {
      clientDataJSON: bufferToBase64url(response.clientDataJSON),
      authenticatorData: bufferToBase64url(response.authenticatorData),
      signature: bufferToBase64url(response.signature),
      userHandle: response.userHandle ? bufferToBase64url(response.userHandle) : undefined,
    },
    clientExtensionResults: credential.getClientExtensionResults(),
  };
}
