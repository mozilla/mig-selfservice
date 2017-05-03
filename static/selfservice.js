function ldrToSlot(ldrname) {
	ri = ldrname.lastIndexOf("-");
	slotval = ldrname.slice(ri + 1);
	slotnum = parseInt(slotval);
	if (isNaN(slotnum)) {
		return undefined;
	}
	return "slot" + slotnum
}

function keyParser(data) {
	for (var i = 1; i < 4; i++) {
		var found = false;
		for (var j = 0; j < data.loaders.length; j++) {
			ldr = data.loaders[j];
			console.log(ldr);
			if (ldr["enabled"] == false) {
				continue;
			}
			ri = ldr["name"].lastIndexOf("-");
			slotval = ldr["name"].slice(ri + 1);
			if (parseInt(slotval) == i) {
				var slotid = "slot" + i;
				t = $("#" + slotid).find("td");
				t.eq(1).html("Assigned");
				t.eq(2).html("<a id=\"" + slotid + "\" href=\"#\">Remove</a>").on("click.rem", removeFunc(slotid));
				found = true;
				break;
			}
		}
		if (found) {
			continue;
		}
		var slotid = "slot" + i;
		t = $("#" + slotid).find("td");
		t.eq(1).html("Not set");
		t.eq(2).html("<a id=\"" + slotid + "\" href=\"#\">Generate key</a>").on("click.gen", generateFunc(slotid));
	}
}

function removeFunc(slotid) {
	return function() {
		$.ajax({
			url: "/delkey",
			type: "post",
			dataType: "text",
			contentType: "application/json",
			data: JSON.stringify({ "slot": slotid }),
			success: loadKeys,
			error: function(xhr, status, error) {
				alert(error);
			}
		});
	}
}

function generateFunc(slotid) {
	return function() {
		$.ajax({
			url: "/newkey",
			type: "post",
			dataType: "json",
			contentType: "application/json",
			data: JSON.stringify({ "slot": slotid }),
			success: showInitialKey,
			error: function(xhr, status, error) {
				alert(error);
			}
		});
	}
}

function showInitialKey(data, textstat, xhr) {
	var slotid = ldrToSlot(data["name"]);
	var keyval = data["prefix"] + data["key"];
	t = $("#" + slotid).find("td");
	t.eq(1).html(keyval);
	t.eq(2).html("Created");
}

function loadKeys() {
	for (var i = 1; i < 4; i++) {
		var slotid = "slot" + i;
		$("#" + slotid).find("td").eq(2).off("click.gen");
		$("#" + slotid).find("td").eq(2).off("click.rem");
	}
	$.ajax({url: "/keystatus", success: keyParser});
}

$(document).ready(function() {
	loadKeys();
});
