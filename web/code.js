'use strict'

const statusOk = 200;
const statusUnauthorized = 401;
const statusForbidden = 403;

//
// Helper functions.
//

function initialize() {
	var elements = $('select.select-score');
	elements.html('');
	for (var i = 0; i <= 10; i++) {
		elements.append($('<option>', {
			'value': i,
			'text': i
		}));
	}
}

function switchToPage(page) {
	$('main').css('display', 'none');
	$('#page_' + page).css('display', 'block');

	if (page === 'games') {
		setTimeout(updateUserList, 0);
		setTimeout(updateGameList, 0);
	}

	if (page === 'addgame') {
		setTimeout(updateUserList, 0);
	}
}

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
// Functions for fetching UI elements from server.
//

async function updateGameList() {
	var token = getToken();
	if (token === null) {
		// We're not logged in. Skip it.
		return;
	}

	var result;
	var data = { 'Token': token }
	try {
		result = await $.post('/api/v1/games', JSON.stringify(data));
	} catch (ex) {
		handleAjaxException(ex);
	}

	var table = $('#games_table');
	table.html('<thead><tr><th>Team Orange</th><th>Team Black</th><th>Result</th></tr></thead>');
	var tbody = table.append($('<tbody>'));
	result.forEach(function (game) {
		var col0 = $('<td>').text(
			`${game.Front1.First} ${game.Front1.Last[0]}. + ${game.Back1.First} ${game.Back1.Last[0]}.`);
		var col1 = $('<td>').text(
			`${game.Front2.First} ${game.Front2.Last[0]}. + ${game.Back2.First} ${game.Back2.Last[0]}.`);
		var col2 = $('<td>').text(
			`${game.Score1} : ${game.Score2}`);

		if (game.Score1 > game.Score2) {
			col0.wrapInner('<strong>');
		}

		if (game.Score2 > game.Score1) {
			col1.wrapInner('<strong>');
		}

		if (Math.abs(game.Score2 - game.Score1) > 2) {
			col2.wrapInner('<strong>');
		}

		var row = $('<tr>');
		row.append(col0);
		row.append(col1);
		row.append(col2);
		tbody.append(row);
	});
}

async function updateUserList() {
	var token = getToken();
	if (token === null) {
		// We're not logged in. Skip it.
		return;
	}

	var result;
	var data = { 'Token': token }
	try {
		result = await $.post('/api/v1/users', JSON.stringify(data));
	} catch (ex) {
		handleAjaxException(ex);
	}

	result.sort(function(a, b) {
		if (a.First < b.First) { return -1; }
		if (a.First > b.First) { return  1; }
		if (a.Last  < b.Last)  { return -1; }
		if (a.Last  > b.Last)  { return  1; }
		return 0;
	});

	var elements = $('select.select-player');
	elements.html('<option value="">-</option>');
	result.forEach(function(user) {
		elements.append($('<option>', {
			'value': user.User,
			'text': `${user.First} ${user.Last}`
		}));
	});

	result.sort(function(a, b) {
		if (a.Elo < b.Elo) { return  1; }
		if (a.Elo > b.Elo) { return -1; }
		return 0;
	});

	var table = $('#users_table');
	table.html('<thead><tr><th>Player</th><th>Elo Score</th></tr></thead>');
	var tbody = table.append($('<tbody>'));
	result.forEach(function(user) {
		var row = $('<tr>');
		row.append($('<td>').text(
			`${user.First} ${user.Last}`));
		row.append($('<td>').text(
			`${user.Elo.toFixed(1)}`));
		tbody.append(row);
	});
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
		'Token':   getToken(),
		'Front1':  form.front1.value,
		'Back1':   form.back1.value,
		'Front2':  form.front2.value,
		'Back2':   form.back2.value,
		'Score1':  parseInt(form.score1.value, 10),
		'Score2':  parseInt(form.score2.value, 10)
	};

	if (data.Front1 == '' || data.Back1 == '' || data.Front2 == '' || data.Back2 == '') {
		alert('Invalid input: Players must be selected.');
		return;
	}

	if (isNaN(data.Score1) || isNaN(data.Score2)) {
		alert('Invalid input: Scores must be integers.');
		return;
	}

	if (data.Front1 == data.Front2 || data.Front1 == data.Back2 || data.Back1 == data.Front2 || data.Back1 == data.Back2) {
		alert('Two teams must be non-intersecting.');
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
// Initialization.
//

$(document).ready(function() {
	initialize();

	if (getToken() === null) {
		switchToPage('login');
	} else {
		switchToPage('games');
	}

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
