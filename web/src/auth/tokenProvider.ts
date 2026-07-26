type TokenProvider = () => Promise<string | null>;

let provider: TokenProvider = async () => null;

export function registerTokenProvider(p: TokenProvider): void {
  provider = p;
}

export async function getAccessToken(): Promise<string | null> {
  return await provider();
}
