# Admin panel

The first user created on your Opengist instance has access to the Admin panel.

To access the Admin panel:

1. Log in
2. Click your username in the upper right corner
3. Select `Admin`

## Usage

### General

Here you can see some basic information, like Opengist version, alongside some stats.

You can also start some actions like forcing synchronization of gists,
starting garbage collection, etc.

### Users

Here you can see your users and delete them.

### Gists

Here you can see all the gists and some basic informations about them. You also have an option
to delete them.


### Invitations

Here you can create invitation links with some options like limiting the number of signed up
users or setting an expiration date.

> [!Note]
> Invitation links override the `Disable signup` option but not the `Disable login form` option.
>
> Users will see only the OAuth providers when `Disable login form` is enabled.

### Configuration

Here you can change a limited number of settings.

- Disable signup
  - Forbid the creation of new accounts.
- Require login
  - Enforce users to be logged in to see gists.
- Disable login form
  - Forbid logging in via the login form to force using OAuth providers instead.
- Disable Gravatar
  - Disable the usage of Gravatar as an avatar provider.
