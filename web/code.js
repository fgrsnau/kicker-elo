'use strict'

const statusOk = 200;
const statusUnauthorized = 401;
const statusForbidden = 403;

var games = [];
var users = [];

//
// Helper functions.
//

function getToken() {
	return localStorage.getItem('Token');
}

function handleAjaxException(ex) {
	var message;
	var handled = false;

	if (ex.readyState === 4) {
		if (ex.status === statusUnauthorized) {
			localStorage.removeItem('Token');
			switchToPage('login');
			handled = true;
		} else {
			message =
				'Sorry, the server send an error response:\n\n' +
				`${ex.status} ${ex.statusText}\n\n` +
				(ex.responseText !== null ? ex.responseText : '');
		}
	} else {
		message =
			'There was an error sending a request to the server. ' +
			'Either there is a problem with your network connection ' +
			'or the backend server is experiencing problems.';
	}

	if (!handled) {
		alert(message);
		location.assign('/');
		throw ex;
	}
}

function createSwitchHandler(page) {
	return function(event) {
		event.preventDefault();
		switchToPage(page);
	};
}

//
// Functions for fetching information from server.
//

async function fetchEverything() {
	await Promise.all([fetchGameList(), fetchUserList()]);
}

async function fetchGameList() {
	var token = getToken();
	if (token === null) {
		// We're not logged in. Skip it.
		return;
	}

	var data = { 'Token': token }
	try {
		games = await $.post('/api/v1/games', JSON.stringify(data));
	} catch (ex) {
		handleAjaxException(ex);
	}
}

async function fetchUserList() {
	var token = getToken();
	if (token === null) {
		// We're not logged in. Skip it.
		return;
	}

	var data = { 'Token': token }
	try {
		users = await $.post('/api/v1/users', JSON.stringify(data));
	} catch (ex) {
		handleAjaxException(ex);
	}
}

//
// Event handlers.
//

async function handleLogin(event) {
	event.preventDefault();

	var result
	var form = $('#form_login')[0];
	var data = {
		'User': form.username.value,
		'Password': form.password.value,
	};

	try {
		result = await $.post('/api/v1/login', JSON.stringify(data));
	} catch (ex) {
		if (ex.status === 401) {
			alert('Wrong credentials.');
			throw ex;
		} else {
			handleAjaxException(ex);
		}
	}
	localStorage.setItem('Token', result['Token']);
	switchToPage('games');
}

function handleLogout() {
	localStorage.removeItem('Token');
	location.assign('/');
}

async function handleRegister(event) {
	event.preventDefault();

	var result
	var form = $('#form_register')[0];
	var data = {
		'User': form.username.value,
		'Password': form.password.value,
		'First': form.first.value,
		'Last': form.last.value
	};

	if (data.User.length < 2 || data.Password.length < 3 || data.First === '' || data.Last === '') {
		alert('Please fill out the form first');
		return;
	}

	try {
		result = await $.post('/api/v1/register', JSON.stringify(data));
	} catch (ex) {
		if (ex.status == statusForbidden) {
			alert('Your chosen username is already taken.');
			throw ex;
		} else {
			handleAjaxException(ex);
		}
	}
	localStorage.setItem('Token', result['Token']);
	switchToPage('games');
}

async function handleAddGame(event) {
	event.preventDefault();

	var result;
	var form = $('#form_addgame')[0];
	var data = {
		'Token': getToken(),
		'Teams': [
			[form.front1.value, form.back1.value],
			[form.front2.value, form.back2.value]],
		'Scores': [
			parseInt(form.score1.value, 10),
			parseInt(form.score2.value, 10)]
	};

	if (data.Teams[0][0] === '' || data.Teams[0][1] == '' || data.Teams[1][0] == '' || data.Teams[1][1] == '') {
		alert('Invalid input: Players must be selected.');
		return;
	}

	if (isNaN(data.Scores[0]) || isNaN(data.Scores[1])) {
		alert('Invalid input: Scores must be integers.');
		return;
	}

	var overlapping = false;
	data.Teams[0].forEach(function(player0) {
		data.Teams[1].forEach(function(player1) {
			overlapping = overlapping || (player0 == player1);
		});
	});

	if (overlapping) {
		alert('Two teams must be non-overlapping.');
		return;
	}

	try {
		await $.post('/api/v1/add_game', JSON.stringify(data))
	} catch (ex) {
		handleAjaxException(ex);
	}

	switchToPage('games');
}

//
// Update UI elements.
//

function switchToPage(page) {
	$('main').css('display', 'none');
	$('#page_' + page).css('display', 'block');

	var f = null;
	if (page === 'games') {
		f = async function() {
			await fetchEverything();
			updatePageGames();
		};
	} else if (page === 'addgame') {
		f = async function() {
			await fetchEverything();
			updatePageAddGame();
		};
	}

	if (f !== null) {
		setTimeout(f, 0);
	}
}

function updatePageGames() {
	var tmp = [...users];

	tmp.sort(function(a, b) {
		if (a.Elo < b.Elo) { return  1; }
		if (a.Elo > b.Elo) { return -1; }
		return 0;
	});

	var table = $('#users_table');
	table.html('<thead><tr><th>Player</th><th>Elo Score</th><th>Won</th><th>Lost</th><th>Total</th></tr></thead>');
	var tbody = table.append($('<tbody>'));
	tmp.forEach(function(user) {
		var row = $('<tr>').appendTo(tbody);
		row.append($('<td>').text(`${user.First} ${user.Last}`));
		row.append($('<td>').text(`${user.Elo.toFixed(0)}`));
		row.append($('<td>').text(`${user.Won}`));
		row.append($('<td>').text(`${user.Lost}`));
		row.append($('<td>').text(`${user.Games}`));
	});

	var formatName = function(user) {
		return `${user.First} ${user.Last[0]}.`;
	};

	var table = $('#games_table');
	table.html('<thead><tr><th>Team Orange</th><th>Team Black</th><th>Result</th></tr></thead>');
	var tbody = table.append($('<tbody>'));
	games.forEach(function (game) {
		var cols = [
			$('<td>').text(`${formatName(game.Teams[0].Front)} + ${formatName(game.Teams[0].Back)}`),
			$('<td>').text(`${formatName(game.Teams[1].Front)} + ${formatName(game.Teams[1].Back)}`),
			$('<td>').text(`${game.Score[0]} : ${game.Score[1]}`),
		]

		for (var i = 0; i < 2; i++) {
			if (game.Score[i] > game.Score[1-i]) {
				cols[i].wrapInner('<strong>');
			}
		}

		if (Math.abs(game.Score[1] - game.Score[0]) > 2) {
			cols[2].wrapInner('<strong>');
		}

		$('<tr>').append(...cols).appendTo(tbody);
	});
}

function updatePageAddGame() {
	var tmp = [...users];

	tmp.sort(function(a, b) {
		if (a.First < b.First) { return -1; }
		if (a.First > b.First) { return  1; }
		if (a.Last  < b.Last)  { return -1; }
		if (a.Last  > b.Last)  { return  1; }
		return 0;
	});

	var elements = $('select.select-player');
	elements.html('<option value="">-</option>');
	tmp.forEach(function(user) {
		elements.append($('<option>', {
			'value': user.User,
			'text': `${user.First} ${user.Last}`
		}));
	});
}

//
// Initialization.
//

$(document).ready(function() {
	// Create score options in select input fields.
	var elements = $('select.select-score');
	elements.html('');
	for (var i = 0; i <= 10; i++) {
		elements.append($('<option>', {
			'value': i,
			'text': i
		}));
	}

	// If we are not logged in, show the login prompt. Otherwise main screen.
	if (getToken() === null) {
		switchToPage('login');
	} else {
		switchToPage('games');
	}

	// Setup all click handlers.
	$('#login_register').click(createSwitchHandler('register'));
	$('#form_login button[type="submit"]').click(handleLogin);
	$('#form_register button[type="submit"]').click(handleRegister);

	$('#register_login').click(createSwitchHandler('login'));

	$('#games_logout').click(handleLogout);
	$('#games_add').click(createSwitchHandler('addgame'));

	$('#addgame_back').click(createSwitchHandler('games'));
	$('#form_addgame button[type="submit"]').click(handleAddGame);

	$('#js_warning').remove();
});
