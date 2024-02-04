import bodyParser from "body-parser";
import cors from "cors";
import express from "express";
import fs from "fs";
import helmet from "helmet";
import { PORT, FRONTEND, EQUINOX } from "../config";
import database from "./database";
import { Logger } from "./helpers/logger";

let avatarCount = 0;

const app = express();

app.use(bodyParser.json({ limit: '25mb' }));
//app.use(helmet());
app.use(cors({ 
    origin: [FRONTEND, EQUINOX], 
    methods: ["POST", "PUT", "GET", "OPTIONS", "HEAD"], 
    }));
app.disable('x-powered-by');

const init = async () => {
    if (!fs.existsSync("static")) fs.mkdir("static", () => { });

    const folders = ["avatars", "spaces", "avatars/default"];

    for (const folder of folders) {
        fs.mkdir(`static/${folder}`, { recursive: true }, () => { });
    }

    const defaultAvatars = fs.readdirSync(`static/avatars/default`);
    if (defaultAvatars.length < 1) throw new Error("Missing at least one default avatar to use in `static/avatars/default`");
    avatarCount = defaultAvatars.length;
}

const startServer = async () => {
    const routes = fs.readdirSync("src/routes");

    for (const route of routes) {
        app.use(`/${route.replace(".ts", '')}`, require(`./routes/${route}`).default);
    }

    app.listen(PORT, () => {
        Logger.success(`Nebula is listening on port ${PORT}!`);
    });

    app.use((_req, res) => {
        res.status(404).json({ message: "The resource you are looking for does not exist!" });
    });
}

(async () => {
    Logger.start();
    await init();
    await database.init();
    await startServer();
})();

export { avatarCount };