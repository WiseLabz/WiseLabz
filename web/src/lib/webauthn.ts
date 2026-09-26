import {
  startAuthentication,
  startRegistration,
  type PublicKeyCredentialCreationOptionsJSON,
  type PublicKeyCredentialRequestOptionsJSON,
} from '@simplewebauthn/browser';
import type { WebAuthnOptions } from '../api/model';

// go-webauthn wraps the browser options in { publicKey: ... }.
export function registerWebAuthn(options: WebAuthnOptions) {
  return startRegistration({ optionsJSON: options.publicKey as PublicKeyCredentialCreationOptionsJSON });
}

export function authenticateWebAuthn(options: WebAuthnOptions) {
  return startAuthentication({ optionsJSON: options.publicKey as PublicKeyCredentialRequestOptionsJSON });
}

export function isWebAuthnCancel(error: unknown): boolean {
  return error instanceof Error && error.name === 'NotAllowedError';
}
