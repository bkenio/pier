require('dotenv').config();

const cors = require('cors');
const morgan = require('morgan');
const express = require('express');
const bodyParser = require('body-parser');
const responseTime = require('response-time');
const cookieParser = require('cookie-parser');

const app = express();

app.use(cors());
app.use(morgan('dev'));
app.use(responseTime());
app.use(cookieParser());
app.use(bodyParser.json());
app.use(bodyParser.urlencoded({ extended: false }));

app.use('/', require('./routes/root'));
// TODO :: Refactor into plural resource
app.use('/users', require('./routes/users'));
app.use('/posts', require('./routes/posts'));
app.use('/videos', require('./routes/videos'));
app.use('/uploads', require('./routes/uploads'));

app.use((req, res, next) => {
  const error = new Error('not found');
  error.status = 404;
  next(error);
});

app.use((error, req, res, next) => {
  res.status(500).send({
    message: error.message || 'unknown',
    error,
  });
});

module.exports = app;
