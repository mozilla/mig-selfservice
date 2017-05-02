function keyParser(data) {
	for (var i = 1; i < 4; i++) {
		var found = false;
		for (var j = 0; j < data.loaders.length; j++) {
			ldr = data.loaders[j];
			ri = ldr["name"].lastIndexOf("-");
			slotval = ldr["name"].slice(ri + 1);
			if (parseInt(slotval) == i) {
				var slotid = "slot" + i;
				t = $("#" + slotid).find("td");
				t.eq(1).html("Assigned");
				t.eq(2).html("Remove");
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
		t.eq(2).html("Generate key");
	}
}

function loadKeys() {
	$.ajax({url: "/keystatus", success: keyParser});
}

document.addEventListener("DOMContentLoaded", function() {
	loadKeys();
});
