# MailerLite subscription proxy

While MailerLite is "lite" and low-bloat in general, they're not "lite" in any way in the morass of HTML/CSS/JS that they want you
to integrate into your site for a basic sign-up form. I'm not comfortable exposing my visitors to that level of risk of JS
injection, so this provides a way to allow sign-ups without needing to include arbitrary JS on the page.

## Deployment

This is written to be deployed in fly.io, because I've been wanting to test their service for a while, and this is about as simple test-case as you can get.