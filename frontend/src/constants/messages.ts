// All user-facing UI strings live here (English). Swap/extend per locale for i18n later.
export const messages = {
  app: { name: "CaliTrack 3D", tagline: "Plan, track and visualize your calisthenics training." },
  nav: { profile: "Profile", logout: "Log out", login: "Log in", register: "Register" },
  auth: {
    loginTitle: "Welcome back",
    loginDescription: "Log in to continue your training.",
    registerTitle: "Create your account",
    registerDescription: "Start tracking your calisthenics progress.",
    email: "Email",
    password: "Password",
    displayName: "Display name",
    submitLogin: "Log in",
    submitRegister: "Create account",
    submitting: "Please wait...",
    noAccount: "Don't have an account?",
    haveAccount: "Already have an account?",
    checkEmail: "Account created. Check your email to confirm your address, then log in.",
    errors: {
      generic: "Something went wrong. Please try again.",
      missingFields: "Please fill in all required fields.",
      passwordTooShort: "Password must be at least 8 characters.",
    },
  },
  profile: {
    title: "Your profile",
    description: "Account details from your sign-in.",
    email: "Email",
    displayName: "Display name",
    memberSince: "Member since",
    notSet: "Not set",
  },
} as const;
