# LarkSuite Drive

[LarkSuite](https://www.larksuite.com/) (飞书国际版) is an enterprise collaboration platform developed by ByteDance. This backend allows rclone to interact with LarkSuite Drive (云空间).

## Configuration

The initial setup for LarkSuite Drive involves getting an App ID and App Secret from the LarkSuite Open Platform. 

### Create a LarkSuite App

1. Go to [LarkSuite Open Platform](https://open.larksuite.com/)
2. Create a new app
3. Enable the following permissions:
   - `drive:file:read` - Read files from Drive
   - `drive:file:write` - Write files to Drive
   - `drive:folder:read` - Read folder information
   - `drive:folder:write` - Create and manage folders
4. Publish the app to make it available
5. Note down the App ID and App Secret

### rclone Configuration

Here is an example of how to configure a LarkSuite Drive remote:

```bash
rclone config
```

```
No remotes found - make a new one
n) New remote
s) Set configuration password
q) Quit config
n/s/q> n
name> larksuite

Type of storage to configure.
Choose a number from below, or type in your own value
[snip]
XX / LarkSuite Drive
   \ "larksuite"
[snip]
Storage> larksuite

LarkSuite App ID
Get from https://open.larksuite.com/app
app_id> cli_xxxxxxxxxxxxxxxx

LarkSuite App Secret
app_secret> xxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxx

Edit advanced config? (y/n)
y) Yes
n) No (default)
y/n> n

Remote config
--------------------
[larksuite]
type = larksuite
app_id = cli_xxxxxxxxxxxxxxxx
app_secret = xxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxx
--------------------
y) Yes this is OK (default)
e) Edit this remote
d) Delete this remote
y/e/d> y
```

### Configuration Options

#### --larksuite-app-id

**Required**

The App ID from your LarkSuite application.

- Config: `app_id`
- Env Var: `RCLONE_LARKSUITE_APP_ID`
- Type: string

#### --larksuite-app-secret

**Required**

The App Secret from your LarkSuite application.

- Config: `app_secret`
- Env Var: `RCLONE_LARKSUITE_APP_SECRET`
- Type: string

#### --larksuite-root-folder-id

**Optional**

The token of the root folder. If not set, uses the default root.

- Config: `root_folder_id`
- Env Var: `RCLONE_LARKSUITE_ROOT_FOLDER_ID`
- Type: string
- Default: ""

#### --larksuite-encoding

**Optional**

This sets the encoding for the backend.

- Config: `encoding`
- Env Var: `RCLONE_LARKSUITE_ENCODING`
- Type: MultiEncoder
- Default: Display,InvalidUtf8

## Usage Examples

### List files

```bash
rclone ls larksuite:
```

### Copy a file to LarkSuite Drive

```bash
rclone copy /path/to/local/file.txt larksuite:/folder/
```

### Copy a file from LarkSuite Drive

```bash
rclone copy larksuite:/folder/file.txt /path/to/local/
```

### Sync a directory

```bash
rclone sync /path/to/local/folder larksuite:/folder/
```

### Make a new directory

```bash
rclone mkdir larksuite:/new-folder
```

### Remove a file

```bash
rclone delete larksuite:/folder/file.txt
```

## Limitations

- File hashes are not supported (LarkSuite Drive does not provide MD5/SHA1 hashes)
- Modification times cannot be set (read-only)
- Maximum file size for simple upload is 20MB (larger files use resumable upload)
- Some characters in file names may be encoded

## Troubleshooting

### Authentication Errors

If you see authentication errors:
1. Verify your App ID and App Secret are correct
2. Ensure your app has the required permissions
3. Check that your app is published in the LarkSuite Open Platform

### Rate Limiting

LarkSuite API has rate limits. If you encounter rate limiting errors:
- Reduce the number of concurrent transfers (`--transfers`)
- Increase the pacer minimum sleep time

## See Also

- [LarkSuite Open Platform Documentation](https://open.larksuite.com/document/home/index)
- [LarkSuite Drive API Reference](https://open.larksuite.com/document/uAjLw4CM/ukTMukTMukTM/reference/drive-v1/introduction)
