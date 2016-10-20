package html

const IndexPage = `
<h1>Login</h1>
<form method="post" action="/login">
<label for="name">User name</label>
<input type="text" id="name" name="name">
<label for="password">Password</label>
<input type="password" id="password" name="password">
<button type="submit">Login</button>
</form>
`

const InternalPage = `<!DOCTYPE html>
<html lang="en">
<head>
<title>SymCPE Function-as-a-Service</title>
<meta charset="UTF-8">
<style type="text/css" media="screen">
     #wrapper { 
         width: 1000px;
         margin:0 auto;
   }

     #editor_div { 
       margin-left: 15px;
       margin-top: 40px;
       width: 800px;
       height: 400px;
   }

    h1 {
        font:bold 3.5em/0.8em Verdana, Arial, sans-serif;
        color: #555;
        margin-left: 10px;
    }
    
    h2 {
        font: italic 1.1em/0.2em Verdana, Arial, sans-serif;
        color:#666;
        margin-left: 15px;

    }

    ul.tab {
      list-style-type: none;
      margin: 0;
      padding: 0;
      overflow: hidden;
      border: 1px solid #ccc;
      background-color: #f1f1f1;

    }

  ul.tab li {
    float: left;
    font:1em Verdana, Arial, sans-serif;
    color:#333;
  }

  ul.tab li a {
      display: inline-block;
      color: black;
      text-align: center;
      padding: 14px 16px;
      text-decoration: none;
      transition: 0.3s;
      font-size: 17px;
    }

/* Change background color of links on hover */
  ul.tab li a:hover {background-color: #ddd;}

/* Create an active/current tablink class */
  ul.tab li a:focus, .active {background-color: #ccc;}

/* Style the tab content */
  .tabcontent {
      display: none;
      padding: 6px 12px;
      border: 1px solid #ccc;
      border-top: none;
  }

  #Apple  {
         height:600px;
  }

  #Pineapple {
    font-family: Verdana, Arial, sans-serif;
    color:#333;

  }

   #myTextarea {
                position: relative;
                top:430px;
                left:15px;
                width:980px;
                height:100px;
                border-style: solid;
                border-color:#666;
                padding:10px;
                border-radius: 8px;
                font:0.8em Verdana, Arial, sans-serif;
                color:#666;
   }
    
    form  {
         text-align:center;
    }

    button {
          position:relative;
          top: 450px;
          left: -200px;
          align-items: flex-start;
          color:#666;
          background-color: #fff;
          width:100px;
          height: 30px;
          border-radius: 8px;
          display: inline-block;
          font-size:14px;
    }

     hr  {
       position: relative;
       top:460px;
       left: 10px;
       margin:5px;
       color:#666;
     }
     
     .codeuploaded {
                  position: relative;
                  top:460px;
                  left: -420px;
                  font:0.8em Verdana, Arial, sans-serif;
                  color:#666;
     }

</style>
</head>

<body>
  <div id="wrapper">
    
    <header>
    <h1>Go-Kexec</h1>
        <h2>Welcome %s! Thanks for using Go-Kexec</h2>
    </header>

    <div id="content">
      <div id="tab">
        <ul class="tab">
          <li><a href="javascript:void(0)" class="tablinks" onclick="openForm(event, 'Apple')" id="defaultOpen">Apple</a></li>
          <li><a href="javascript:void(0)" class="tablinks" onclick="openForm(event, 'Pineapple')">Pineapple</a></li>
        </ul>
      </div>

      <div id="Apple" class="tabcontent">
        <div id="editor_div">def foo():
    print("Go-kexec is awesome.")

foo()</div>

        <form id="codeForm" action="/create" method="post" enctype="multipart/form-data">
          <input type="text" name="functionName" value="default_function">
          <select name="runtime">
            <option value="python27">Python2.7</option>
          </select>
          <button type="button" onclick="myFunction()">Submit</button>
          <hr>
          <p class="codeuploaded">Code Uploaded:</p>
          <textarea id="myTextarea" name="codeTextarea" style="display:none;">Default value</textarea>
        </form>

      </div>

      <div id="Pineapple" class="tabcontent">
        <h3>Pineapple</h3>
        <p>%s</p>
      </div>
      <div id="logout">
        <a href="/logout">Log Out</a>
      </div>
    </div>


  </div>

<script src="http://d1n0x3qji82z53.cloudfront.net/src-min-noconflict/ace.js" type="text/javascript">
</script>
<script type="text/javascript">

  // Get tab functinons
  
  function openForm(evt, tabName) {

    var i, tabcontent, tablinks;
    tabcontent = document.getElementsByClassName("tabcontent");
    for (i = 0; i < tabcontent.length; i++) {
        tabcontent[i].style.display = "none";
    }
    tablinks = document.getElementsByClassName("tablinks");
    for (i = 0; i < tablinks.length; i++) {
        tablinks[i].className = tablinks[i].className.replace(" active", "");
    }
    document.getElementById(tabName).style.display = "block";
    evt.currentTarget.className += " active";
    }
    document.getElementById("defaultOpen").click();

    // Get form functions
    var editor = ace.edit("editor_div");
    editor.setTheme("ace/theme/monokai");
    editor.getSession().setMode("ace/mode/python");
  
    function myFunction() {
    var textarea = document.getElementById("myTextarea");
    textarea.value = editor.getSession().getValue();
    document.getElementById("codeForm").submit();
    textarea.style.display = "block";
  }

</script>

</body>
</html>
`

const FunctionFailedErrorPage = `
<h1>Function failed. Redirecting...<h1>
%s
`

const FunctionCreatedPage = `
<h1>Function created successfully.<h1>
<button type="button" onclick="history.go(-1);">Back</button>
`

const FunctionCalledPage = `
<h1>Function called successfully.<h1>
<button type="button" onclick="history.go(-1);">Back</button>
`
