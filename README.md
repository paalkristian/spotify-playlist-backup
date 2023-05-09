# Spotify Backup
This Go program fetches all your playlists from Spotify and stores them in JSON files. Move the JSON files to a safe place, and if you lose access to your Spotify account, you can write a program using the Spotify API to restore your playlists.

To run the program:
1. Go to https://developer.spotify.com/dashboard to get a client secret and client ID for the API.
2. Add the client ID and client secret to the .env file.
3. Run go run in your terminal and follow the instructions.

The program will pause for a few seconds after fetching data for a playlist. This is a conservative measure to avoid rate limiting.

# Ideas for New Features
- Store all data in an SQLite database to enable running queries on the dataset.
- Use the Spotify API to restore a playlist.
- Schedule backups.
- Create a REST API and a GUI to schedule and restore backups.