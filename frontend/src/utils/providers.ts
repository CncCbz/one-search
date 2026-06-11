export const providerOptions = [
  { label: 'Exa', value: 'exa' },
  { label: 'You.com', value: 'you' },
  { label: 'Jina', value: 'jina' },
  { label: 'Tavily', value: 'tavily' },
  { label: 'Firecrawl', value: 'firecrawl' },
  { label: 'Serper', value: 'serper' },
  { label: 'Brave Search', value: 'brave' }
]

export const defaultProviders = providerOptions.map((item) => item.value)

export function providerLabel(provider: string) {
  return providerOptions.find((item) => item.value === provider)?.label || provider
}
