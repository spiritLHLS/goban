const usernameKey = 'username'
const passwordKey = 'password'

export function getCredentials() {
  let username = sessionStorage.getItem(usernameKey)
  let password = sessionStorage.getItem(passwordKey)

  if (!username || !password) {
    const legacyUsername = localStorage.getItem(usernameKey)
    const legacyPassword = localStorage.getItem(passwordKey)
    if (legacyUsername && legacyPassword) {
      setCredentials(legacyUsername, legacyPassword)
      username = legacyUsername
      password = legacyPassword
    }
  }

  return { username, password }
}

export function setCredentials(username, password) {
  sessionStorage.setItem(usernameKey, username)
  sessionStorage.setItem(passwordKey, password)
  localStorage.removeItem(usernameKey)
  localStorage.removeItem(passwordKey)
}

export function clearCredentials() {
  sessionStorage.removeItem(usernameKey)
  sessionStorage.removeItem(passwordKey)
  localStorage.removeItem(usernameKey)
  localStorage.removeItem(passwordKey)
}
